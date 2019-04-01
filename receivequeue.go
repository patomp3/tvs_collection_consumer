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

	sms "github.com/patomp3/smsservices"
	"github.com/streadway/amqp"
)

// ReceiveQueue struct...
type ReceiveQueue struct {
	URL       string
	QueueName string
}

// UpdatePayloadRequest for ...
type UpdatePayloadRequest struct {
	OrderTransID string            `json:"order_trans_id"`
	Payload      map[string]string `json:"payload"`
}

// UpdatePayloadResponse for ..
type UpdatePayloadResponse struct {
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
			dbPED := sms.New(cfg.dbPED)
			bResult := dbPED.ExecuteStoreProcedure("begin PK_WFA_CORE.GetPayloadData(:1,:2); end;", orderTransID, sql.Out{Dest: &orderResult})
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
				var res OrderResponse
				req := OrderRequest{payload["tvscustomer"], payload["actioncode"], payload["activityreasoncode"], orderTransID}

				// Get ServiceCode from ActionCode & ActivityReasonCode
				var rs driver.Rows
				dbATB2 := sms.New(cfg.dbATB2)
				bResult = dbATB2.ExecuteStoreProcedure("begin PK_IBS_CCBS_ORDER.BG_CCBS_ORDER_SERVICECODE(:1,:2,:3); end;", req.ActionCode,
					req.ActivityReasonCode, sql.Out{Dest: &rs})
				if bResult && rs != nil {
					values := make([]driver.Value, len(rs.Columns()))

					// ok
					for rs.Next(values) == nil {
						log.Printf("## Service Code = %s", values[0].(string))
						switch serviceCode := values[0].(string); serviceCode {
						case "CANCEL":
							res, _ = req.Cancel()
							log.Printf("## Cancel Result = %s %d, %s", res.Status, res.ErrorCode, res.ErrorDescription)
						case "DISCONNECT":
							res, _ = req.Disconnect("Disconnect")
							log.Printf("## Disconnect Result = %s %d, %s", res.Status, res.ErrorCode, res.ErrorDescription)
						case "DISCONNECTPTP", "CANCELPTP":
							res, _ = req.Disconnect("DisconnectPTP")
							log.Printf("## DisconnectPTP Result = %s %d, %s", res.Status, res.ErrorCode, res.ErrorDescription)
						case "RECONNECT":
							res, _ = req.Reconnect("Reconnect")
							log.Printf("## Reconnect Result = %s %d, %s", res.Status, res.ErrorCode, res.ErrorDescription)
						case "RECONPTP":
							res, _ = req.Reconnect("ReconnectPTP")
							log.Printf("## ReconnectPTP Result = %s %d, %s", res.Status, res.ErrorCode, res.ErrorDescription)
						}

						if res.IsSuspend == true {
							sentToSuspendSubscriber(orderTransID, payload)
						}

						// Notify result to wfa core
						notifyStatus := "Z"
						if res.ErrorCode != 0 {
							notifyStatus = "E"
						}
						resStr, _ := json.Marshal(res)
						result := UpdateRequest{orderTransID, orderID, notifyStatus, strconv.Itoa(res.ErrorCode), res.ErrorDescription, string(resStr)}
						result.NotifyResult()
						_ = result

					}
				}
			}
			// End Process
		}
	}()

	log.Printf("## [*] Waiting for messages. To exit press CTRL+C")
	<-forever
}

func sentToSuspendSubscriber(orderTransID string, payload map[string]string) bool {
	var myReturn bool

	var req UpdatePayloadRequest
	var res UpdatePayloadResponse

	log.Printf("## Update payload to suspend sub to CCBS")

	req.OrderTransID = orderTransID
	req.Payload = payload
	//Update Payload to send suspend at next queue
	req.Payload["suspendsubscriber"] = "Y"

	reqPost, _ := json.Marshal(req)

	response, err := http.Post(cfg.updatePayloadURL, "application/json", bytes.NewBuffer(reqPost))
	if err != nil {
		log.Printf("The HTTP request failed with error %s", err)
	} else {
		data, _ := ioutil.ReadAll(response.Body)
		//fmt.Println(string(data))
		//myReturn = json.Unmarshal(string(data))
		err = json.Unmarshal(data, &res)
		if err != nil {
			//panic(err)
			log.Printf("The HTTP response failed with error %s", err)
			myReturn = false
		} else {
			log.Printf("## Result >> %v", res)
		}
	}

	if res.OrderTransID != "" && res.ErrorCode == "0" {
		myReturn = true
	}

	return myReturn
}
