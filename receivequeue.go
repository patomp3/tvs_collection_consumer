package main

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"

	"github.com/streadway/amqp"
)

// ReceiveQueue struct...
type ReceiveQueue struct {
	URL       string
	QueueName string
}

// UpdateRequest for ...
type UpdateRequest struct {
	OrderTransID    string `json:"order_trans_id"`
	OrderID         string `json:"order_id"`
	Status          string `json:"status"`
	ErrorCode       string `json:"error_code"`
	ErrorDesc       string `json:"error_desc"`
	ResponseMessage string `json:"response_message"`
}

// UpdateResponse for ..
type UpdateResponse struct {
	OrderTransID     string `json:"order_trans_id"`
	ErrorCode        string `json:"error_code"`
	ErrorDescription string `json:"error_description"`
}

func failOnError(err error, msg string) {
	if err != nil {
		fmt.Printf("%s: %s", msg, err)
	}
}

// Close for
func (r ReceiveQueue) Close() {
	//q.conn.Close()
	//q.ch.Close()
}

// Connect for
func (r ReceiveQueue) Connect() *amqp.Channel {
	conn, err := amqp.Dial(r.URL)
	//defer conn.Close()
	if err != nil {
		failOnError(err, "Failed to connect to RabbitMQ")
		return nil
	}

	ch, err := conn.Channel()
	//defer ch.Close()
	if err != nil {
		failOnError(err, "Failed to open a channel")
		return nil
	}

	return ch
}

// Receive for receive message from queue
func (r ReceiveQueue) Receive(ch *amqp.Channel) {

	/*conn, err := amqp.Dial(q.URL)
	defer conn.Close()
	if err != nil {
		failOnError(err, "Failed to connect to RabbitMQ")
		return false
	}

	ch, err := conn.Channel()
	defer ch.Close()
	if err != nil {
		failOnError(err, "Failed to open a channel")
		return false
	}*/

	q, err := ch.QueueDeclarePassive(
		r.QueueName, // name
		true,        // durable
		false,       // delete when unused
		false,       // exclusive
		false,       // no-wait
		nil,         // arguments
	)
	if err != nil {
		failOnError(err, "Failed to declare a queue")
	}

	msgs, err := ch.Consume(
		q.Name, // routing key
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		failOnError(err, "Failed to publish a message")

	}

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			orderTransID := string(d.Body)
			orderID := d.MessageId

			log.Printf("## Received Order Trans Id : %s", orderTransID)
			log.Printf("## >> Id : %s", orderID)

			//TODO : Process for consumer to
			var orderResult driver.Rows
			var payload map[string]string
			isError := 0
			//log.Printf("Execute Store return cursor")
			bResult := ExecuteStoreProcedure("QED", "begin PK_WFA_CORE.GetPayloadData(:1,:2); end;", orderTransID, sql.Out{Dest: &orderResult})
			if bResult && orderResult != nil {
				values := make([]driver.Value, len(orderResult.Columns()))
				if orderResult.Next(values) == nil {
					payloadStr := values[1].(string)
					log.Printf("payload = %s", payloadStr)

					err := json.Unmarshal([]byte(payloadStr), &payload)
					if err != nil {
						//panic(err)
						isError = 1
					}
				}
			}

			// not error
			if isError == 0 {
				//Read Json message
				var req OrderRequest
				req.Init(payload["tvscustomer"], payload["actioncode"], payload["activityreasoncode"])

				// Get ServiceCode from ActionCode & ActivityReasonCode
				var rs driver.Rows
				bResult = ExecuteStoreProcedure(cfg.dbATB2, "begin PK_IBS_CCBS_ORDER.BG_CCBS_ORDER_SERVICECODE(:1,:2,:3); end;", req.ActionCode,
					req.ActivityReasonCode, sql.Out{Dest: &rs})
				if bResult && rs != nil {
					values := make([]driver.Value, len(rs.Columns()))

					// ok
					for rs.Next(values) == nil {
						var res OrderResponse

						log.Printf("## Service Code = %s", values[0].(string))
						switch serviceCode := values[0].(string); serviceCode {
						case "CANCEL", "CANCELPTP":
							res, _ = Cancel(req)
							log.Printf("## Cancel Result = %s %d, %s", res.Status, res.ErrorCode, res.ErrorDescription)
						case "DISCONNECT":
							res, _ = Disconnect(req)
							log.Printf("## Disconnect Result = %s %d, %s", res.Status, res.ErrorCode, res.ErrorDescription)
						case "DISCONNECTPTP":
							res, _ = DisconnectPTP(req)
							log.Printf("## DisconnectPTP Result = %s %d, %s", res.Status, res.ErrorCode, res.ErrorDescription)
						case "RECONNECT":
							res, _ = Reconnect(req)
							log.Printf("## Reconnect Result = %s %d, %s", res.Status, res.ErrorCode, res.ErrorDescription)
						case "RECONPTP":
							res, _ = ReconnectPTP(req)
							log.Printf("## ReconnectPTP Result = %s %d, %s", res.Status, res.ErrorCode, res.ErrorDescription)
						}

						//TODO : Notify result to wfa core
						var result UpdateRequest
						result.OrderTransID = orderTransID
						result.OrderID = orderID
						result.Status = "Z"
						if res.ErrorCode != 0 {
							result.Status = "E"
						}
						///result.Status = "Z"
						result.ErrorCode = strconv.Itoa(res.ErrorCode)
						result.ErrorDesc = res.ErrorDescription
						notifyResult(result)
						// End Notify Result
					}
				}
			}
			// End Process
		}
	}()

	log.Printf("## [*] Waiting for messages. To exit press CTRL+C")
	<-forever
}

func notifyResult(req UpdateRequest) UpdateResponse {
	var resultRes UpdateResponse

	reqPost, _ := json.Marshal(req)

	response, err := http.Post(cfg.updateOrderURL, "application/json", bytes.NewBuffer(reqPost))
	if err != nil {
		log.Printf("The HTTP request failed with error %s", err)
	} else {
		data, _ := ioutil.ReadAll(response.Body)
		//fmt.Println(string(data))
		//myReturn = json.Unmarshal(string(data))
		err = json.Unmarshal(data, &resultRes)
		if err != nil {
			//panic(err)
			log.Printf("The HTTP response failed with error %s", err)
		} else {
			log.Printf("## Result >> %v", resultRes)
		}
	}

	return resultRes
}
