package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"

	smslog "github.com/patomp3/smslogs"
)

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

// NotifyResult for notify to wfa core
func (r UpdateRequest) NotifyResult() UpdateResponse {
	var result UpdateResponse

	log.Printf("## NotifyResult: Request = %v", r)

	reqPost, _ := json.Marshal(r)

	response, err := http.Post(cfg.updateOrderURL, "application/json", bytes.NewBuffer(reqPost))
	if err != nil {
		log.Printf("The HTTP request failed with error %s", err)
	} else {
		data, _ := ioutil.ReadAll(response.Body)
		//fmt.Println(string(data))
		//myReturn = json.Unmarshal(string(data))
		err = json.Unmarshal(data, &result)
		if err != nil {
			//panic(err)
			log.Printf("The HTTP response failed with error %s", err)
		} else {
			log.Printf("## Result >> %v", result)
		}
	}

	log.Printf("## NotifyResult: Response = %v", result)

	// Write log to stdoutput
	if cfg.log == "Y" {
		appFunc := "TVS_COLLECTION-NotifyResult"
		jsonReq, _ := json.Marshal(r)
		jsonRes, _ := json.Marshal(result)

		mLog := smslog.New(cfg.appName)
		mLog.OrderDate = ""
		mLog.OrderNo = r.OrderTransID
		mLog.OrderType = ""
		mLog.TVSNo = ""
		tag := []string{cfg.env, cfg.appName + "-" + appFunc, "INFO"}
		mLog.Tags = tag
		mLog.PrintLog(smslog.INFO, appFunc, r.OrderTransID, string(jsonReq), string(jsonRes))
		// End Write log
	}

	return result
}
