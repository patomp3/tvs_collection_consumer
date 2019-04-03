package main

import (
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type appConfig struct {
	queueName string
	queueURL  string

	dbICC         string
	dbPED         string
	dbATB2        string
	disconnectURL string
	reconnectURL  string
	cancelURL     string

	updateOrderURL   string
	updatePayloadURL string

	ccbsAccountURL string

	env     string
	appName string
	log     string
	debug   string
}

var cfg appConfig

func main() {

	// For no assign parameter env. using default to Test
	var env string
	if len(os.Args) > 1 {
		env = strings.ToLower(os.Args[1])
	} else {
		env = "development"
	}

	// Load configuration
	viper.SetConfigName("app")    // no need to include file extension
	viper.AddConfigPath("config") // set the path of your config file
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("## Config file not found. >> %s\n", err.Error())
	} else {
		// read config file
		cfg.queueName = viper.GetString(env + ".queuename")
		cfg.queueURL = viper.GetString(env + ".queueurl")
		cfg.dbICC = viper.GetString(env + ".DBICC")
		cfg.dbATB2 = viper.GetString(env + ".DBATB2")
		cfg.dbPED = viper.GetString(env + ".DBPED")
		cfg.disconnectURL = viper.GetString(env + ".disconnecturl")
		cfg.reconnectURL = viper.GetString(env + ".reconnecturl")
		cfg.cancelURL = viper.GetString(env + ".cancelurl")
		cfg.updateOrderURL = viper.GetString(env + ".updateorderurl")
		cfg.updatePayloadURL = viper.GetString(env + ".updatepayloadurl")
		cfg.ccbsAccountURL = viper.GetString(env + ".ccbsaccountserviceurl")

		cfg.env = viper.GetString("env")
		cfg.appName = viper.GetString("appName")
		cfg.debug = viper.GetString("debugMode")
		cfg.log = viper.GetString("logMode")

		if cfg.debug != "" {
			file, err := os.OpenFile(cfg.debug+time.Now().Format("2006-01-02")+".log", os.O_CREATE|os.O_APPEND, 0644)
			if err != nil {
				log.Fatal(err)
			}

			defer file.Close()

			log.SetOutput(file)
		} else {
			log.SetOutput(ioutil.Discard)
		}

		log.Printf("##### Service Consumer Started #####")
		log.Printf("## Loading Configuration")
		log.Printf("## Env\t= %s", env)
	}

	q := ReceiveQueue{cfg.queueURL, cfg.queueName}
	ch := q.Connect()
	q.Receive(ch)

	// test function
	/*orderTransID := "c7e630f5-7e26-a9a1-96e0-b83874a623cb"
	var orderResult driver.Rows
	var payload map[string]string
	//isError := 0
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
				//isError = 1
			}

			log.Printf("%v", payloadStr)
		}
	}

	var res OrderResponse
	req := OrderRequest{payload["tvscustomer"], payload["actioncode"], payload["activityreasoncode"], orderTransID}

	res, _ = req.Reconnect("Reconnect")

	log.Printf("## Reconnect Result = %s %d, %s", res.Status, res.ErrorCode, res.ErrorDescription)*/
}
