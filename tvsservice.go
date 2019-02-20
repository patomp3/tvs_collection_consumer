package main

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// OrderRequest Object
type OrderRequest struct {
	TVSCustomer        string `json:"TVSCustomer"`
	ActionCode         string `json:"ActionCode"`
	ActivityReasonCode string `json:"ActivityReasonCode"`
}

// Init add initial value
func (o *OrderRequest) Init(tvsCustomer string, actionCode string, activityReasonCode string) {
	o.TVSCustomer = tvsCustomer
	o.ActionCode = actionCode
	o.ActivityReasonCode = activityReasonCode
}

// OrderResponse Object
type OrderResponse struct {
	ErrorCode        int    `json:"ErrorCode"`
	ErrorDescription string `json:"ErrorDescription"`
	Status           string `json:"Status"`
}

// ServiceRequest for recon struct
type ServiceRequest struct {
	ByUser struct {
		ByChannel string `json:"byChannel"`
		ByUser    string `json:"byUser"`
	} `json:"ByUser"`
	Customer struct {
		CustomerID int `json:"CustomerID"`
	} `json:"Customer"`
	Product struct {
		Product []struct {
			ProductID int `json:"ProductId"`
		} `json:"Product"`
	} `json:"Product"`
	Reason int `json:"Reason"`
	Target struct {
		Target     int    `json:"Target"`
		TargetDate string `json:"TargetDate"`
	} `json:"Target"`
}

// ServiceResult for recon result
type ServiceResult struct {
	ErrorCode   int    `json:"ErrorCode"`
	ErrorDesc   string `json:"ErrorDesc"`
	ResultValue string `json:"ResultValue"`
	ProductID   int    `json:"ProductId"`
}

// GetBalance for ccbs getAccountBalance
type GetBalance struct {
	XMLName xml.Name `xml:"Envelope"`
	Text    string   `xml:",chardata"`
	S       string   `xml:"s,attr"`
	Body    struct {
		Text                      string `xml:",chardata"`
		GetAccountBalanceResponse struct {
			Text                    string `xml:",chardata"`
			Xmlns                   string `xml:"xmlns,attr"`
			GetAccountBalanceResult struct {
				Text      string `xml:",chardata"`
				A         string `xml:"a,attr"`
				I         string `xml:"i,attr"`
				ErrorCode struct {
					Text string `xml:",chardata"`
				} `xml:"ErrorCode"`
				ErrorDesc struct {
					Text string `xml:",chardata"`
				} `xml:"ErrorDesc"`
				ErrorDetail struct {
					Text string `xml:",chardata"`
					Nil  string `xml:"nil,attr"`
				} `xml:"ErrorDetail"`
				TransInfo struct {
					Text    string `xml:",chardata"`
					Channel struct {
						Text string `xml:",chardata"`
					} `xml:"Channel"`
					ResubmitBy struct {
						Text string `xml:",chardata"`
						Nil  string `xml:"nil,attr"`
					} `xml:"ResubmitBy"`
					ResubmitTime struct {
						Text string `xml:",chardata"`
						Nil  string `xml:"nil,attr"`
					} `xml:"ResubmitTime"`
					SequenceId struct {
						Text string `xml:",chardata"`
						Nil  string `xml:"nil,attr"`
					} `xml:"SequenceId"`
					TransId struct {
						Text string `xml:",chardata"`
					} `xml:"TransId"`
					UserId struct {
						Text string `xml:",chardata"`
					} `xml:"UserId"`
				} `xml:"TransInfo"`
				SearchResult struct {
					Text            string `xml:",chardata"`
					B               string `xml:"b,attr"`
					DummyFieldField struct {
						Text string `xml:",chardata"`
					} `xml:"dummyFieldField"`
					ArBalanceField struct {
						Text string `xml:",chardata"`
					} `xml:"arBalanceField"`
					UnappliedAmountField struct {
						Text string `xml:",chardata"`
					} `xml:"unappliedAmountField"`
				} `xml:"SearchResult"`
			} `xml:"GetAccountBalanceResult"`
		} `xml:"GetAccountBalanceResponse"`
	} `xml:"Body"`
}

// Cancel Service
func Cancel(req OrderRequest) (OrderResponse, error) {
	var result OrderResponse

	log.Printf("## Process Cancel Service")

	// get product to Cancel
	var rs driver.Rows
	var productID int
	var commercialProductID int
	var reason int
	bResult := ExecuteStoreProcedure(cfg.dbICC, "begin PK_CCBS_COLLECTION.GetCancelAgreementDetail(:1,:2,:3,:4); end;", req.TVSCustomer, req.ActionCode,
		req.ActivityReasonCode, sql.Out{Dest: &rs})
	if bResult && rs != nil {
		values := make([]driver.Value, len(rs.Columns()))
		// ok
		haveProduct := false
		for rs.Next(values) == nil {
			haveProduct = true

			productID, _ = strconv.Atoi(strconv.FormatInt(values[0].(int64), 10))
			commercialProductID, _ = strconv.Atoi(strconv.FormatInt(values[1].(int64), 10))
			reason, _ = strconv.Atoi(values[2].(string))

			log.Printf("## Product Cancel = %d %d %d", productID, commercialProductID, reason)

			// Call Service to Cancel
			var serviceRes ServiceResult

			strReq := "{\r\n\t\"ByUser\":{\r\n\t\t\"byChannel\":\"#BYCHANNEL\",\r\n\t\t\"byUser\":\"#BYUSER\"\r\n\t},\r\n\t\"Customer\":{\r\n\t\t\"CustomerID\":#CUSTOMERID\r\n\t},\r\n\t\"Product\":{\r\n\t\t\"Product\":[{\r\n\t\t\t\"ProductId\":#PRODUCTID\t\t\r\n\t\t}]\r\n\t},\r\n\t\"Reason\":#REASON,\r\n\t\"Target\":{\r\n\t\t\"Target\":#TARGET,\r\n\t\t\"TargetDate\":\"\"\r\n\t}\r\n}"
			strReq = strings.Replace(strReq, "#BYCHANNEL", "", -1)
			strReq = strings.Replace(strReq, "#BYUSER", "BILL-COL", -1)
			strReq = strings.Replace(strReq, "#CUSTOMERID", req.TVSCustomer, -1)
			strReq = strings.Replace(strReq, "#PRODUCTID", strconv.Itoa(productID), -1)
			strReq = strings.Replace(strReq, "#REASON", strconv.Itoa(reason), -1)
			strReq = strings.Replace(strReq, "#TARGET", "0", -1)

			req := []byte(strReq)

			response, err := http.Post(cfg.cancelURL, "application/json", bytes.NewBuffer(req))
			if err != nil {
				log.Printf("The HTTP request failed with error %s", err)
			} else {
				data, _ := ioutil.ReadAll(response.Body)
				//fmt.Println(string(data))
				//myReturn = json.Unmarshal(string(data))
				err = json.Unmarshal(data, &serviceRes)
				if err != nil {
					//panic(err)
					log.Printf("The HTTP response failed with error %s", err)
				} else {
					log.Printf("## Result >> %d %d %s", serviceRes.ProductID, serviceRes.ErrorCode, serviceRes.ErrorDesc)
					result.Status = serviceRes.ResultValue
					result.ErrorCode = serviceRes.ErrorCode
					result.ErrorDescription = serviceRes.ErrorDesc
				}
			}
		}

		if !haveProduct {
			result.Status = "true"
			result.ErrorCode = 0
			result.ErrorDescription = "No products to cancel"
		}
	}

	return result, nil
}

// Disconnect Service
func Disconnect(req OrderRequest) (OrderResponse, error) {
	var result OrderResponse

	log.Printf("## Process Disconnecct Service")

	// get product to disconnect
	var rs driver.Rows
	var productID int
	var commercialProductID int
	var reason int
	bResult := ExecuteStoreProcedure(cfg.dbICC, "begin PK_CCBS_COLLECTION.GetDisconAgreementDetail(:1,:2,:3,:4); end;", req.TVSCustomer, req.ActionCode,
		req.ActivityReasonCode, sql.Out{Dest: &rs})
	if bResult && rs != nil {
		values := make([]driver.Value, len(rs.Columns()))
		// ok
		haveProduct := false
		for rs.Next(values) == nil {
			haveProduct = true

			productID, _ = strconv.Atoi(strconv.FormatInt(values[0].(int64), 10))
			commercialProductID, _ = strconv.Atoi(strconv.FormatInt(values[1].(int64), 10))
			reason, _ = strconv.Atoi(values[2].(string))

			log.Printf("## Product Disconnect = %d %d %d", productID, commercialProductID, reason)

			// Call Service to disconnect
			var serviceRes ServiceResult

			strReq := "{\r\n\t\"ByUser\":{\r\n\t\t\"byChannel\":\"#BYCHANNEL\",\r\n\t\t\"byUser\":\"#BYUSER\"\r\n\t},\r\n\t\"Customer\":{\r\n\t\t\"CustomerID\":#CUSTOMERID\r\n\t},\r\n\t\"Product\":{\r\n\t\t\"Product\":[{\r\n\t\t\t\"ProductId\":#PRODUCTID\t\t\r\n\t\t}]\r\n\t},\r\n\t\"Reason\":#REASON,\r\n\t\"Target\":{\r\n\t\t\"Target\":#TARGET,\r\n\t\t\"TargetDate\":\"\"\r\n\t}\r\n}"
			strReq = strings.Replace(strReq, "#BYCHANNEL", "", -1)
			strReq = strings.Replace(strReq, "#BYUSER", "BILL-COL", -1)
			strReq = strings.Replace(strReq, "#CUSTOMERID", req.TVSCustomer, -1)
			strReq = strings.Replace(strReq, "#PRODUCTID", strconv.Itoa(productID), -1)
			strReq = strings.Replace(strReq, "#REASON", strconv.Itoa(reason), -1)
			strReq = strings.Replace(strReq, "#TARGET", "0", -1)

			req := []byte(strReq)

			response, err := http.Post(cfg.disconnectURL, "application/json", bytes.NewBuffer(req))
			if err != nil {
				log.Printf("The HTTP request failed with error %s", err)
			} else {
				data, _ := ioutil.ReadAll(response.Body)
				//fmt.Println(string(data))
				//myReturn = json.Unmarshal(string(data))
				err = json.Unmarshal(data, &serviceRes)
				if err != nil {
					//panic(err)
					log.Printf("The HTTP response failed with error %s", err)
				} else {
					log.Printf("## Result >> %d %d %s", serviceRes.ProductID, serviceRes.ErrorCode, serviceRes.ErrorDesc)
					result.Status = serviceRes.ResultValue
					result.ErrorCode = serviceRes.ErrorCode
					result.ErrorDescription = serviceRes.ErrorDesc
				}
			}
		}

		if !haveProduct {
			result.Status = "true"
			result.ErrorCode = 0
			result.ErrorDescription = "No products to disconnect"
		}
	}

	// Disconnect all child account
	var rsChild driver.Rows
	var childAccount int
	bResult = ExecuteStoreProcedure(cfg.dbICC, "begin PK_CCBS_COLLECTION.GetChildAccount(:1,:2,:3,:4); end;", req.TVSCustomer, req.ActionCode,
		req.ActivityReasonCode, sql.Out{Dest: &rsChild})
	if bResult && rsChild != nil {
		values := make([]driver.Value, len(rs.Columns()))
		log.Printf("## Process to disconnecct child account")

		for rsChild.Next(values) == nil {
			childAccount, _ = strconv.Atoi(strconv.FormatInt(values[0].(int64), 10))

			log.Printf("## Product Disconnect = %d %d %d", productID, commercialProductID, reason)

			// Call Service to disconnect
			var serviceRes ServiceResult

			strReq := "{\r\n\t\"ByUser\":{\r\n\t\t\"byChannel\":\"#BYCHANNEL\",\r\n\t\t\"byUser\":\"#BYUSER\"\r\n\t},\r\n\t\"Customer\":{\r\n\t\t\"CustomerID\":#CUSTOMERID\r\n\t},\r\n\t\"Reason\":#REASON,\r\n\t\"Target\":{\r\n\t\t\"Target\":#TARGET,\r\n\t\t\"TargetDate\":\"\"\r\n\t}\r\n}"
			strReq = strings.Replace(strReq, "#BYCHANNEL", "", -1)
			strReq = strings.Replace(strReq, "#BYUSER", "BILL-COL", -1)
			strReq = strings.Replace(strReq, "#CUSTOMERID", strconv.Itoa(childAccount), -1)
			strReq = strings.Replace(strReq, "#REASON", strconv.Itoa(reason), -1)
			strReq = strings.Replace(strReq, "#TARGET", "0", -1)

			req := []byte(strReq)

			response, err := http.Post(cfg.disconnectURL, "application/json", bytes.NewBuffer(req))
			if err != nil {
				log.Printf("The HTTP request failed with error %s", err)
			} else {
				data, _ := ioutil.ReadAll(response.Body)
				//fmt.Println(string(data))
				//myReturn = json.Unmarshal(string(data))
				err = json.Unmarshal(data, &serviceRes)
				if err != nil {
					//panic(err)
					log.Printf("The HTTP response failed with error %s", err)
				} else {
					log.Printf("## Result >> %d %d %s", serviceRes.ProductID, serviceRes.ErrorCode, serviceRes.ErrorDesc)
					result.Status = serviceRes.ResultValue
					result.ErrorCode = serviceRes.ErrorCode
					result.ErrorDescription = serviceRes.ErrorDesc
				}
			}
			if childAccount == 0 {
				log.Printf("## No account child to disconnect")
			}
		}
	}
	// End Disconnect all child

	return result, nil
}

// DisconnectPTP Service
func DisconnectPTP(req OrderRequest) (OrderResponse, error) {
	var result OrderResponse

	log.Printf("## Process DisconnectPTP Service")

	// get product to disconnect
	var rs driver.Rows
	var productID int
	var commercialProductID int
	var reason int
	bResult := ExecuteStoreProcedure(cfg.dbICC, "begin PK_CCBS_COLLECTION.GetDisconPTPAgreementDetail(:1,:2,:3,:4); end;", req.TVSCustomer, req.ActionCode,
		req.ActivityReasonCode, sql.Out{Dest: &rs})
	if bResult && rs != nil {
		values := make([]driver.Value, len(rs.Columns()))
		// ok
		haveProduct := false
		for rs.Next(values) == nil {
			haveProduct = true

			productID, _ = strconv.Atoi(strconv.FormatInt(values[0].(int64), 10))
			commercialProductID, _ = strconv.Atoi(strconv.FormatInt(values[1].(int64), 10))
			reason, _ = strconv.Atoi(values[2].(string))

			log.Printf("## Product DisconnectPTP = %d %d %d", productID, commercialProductID, reason)

			// Call Service to disconnect
			var serviceRes ServiceResult

			strReq := "{\r\n\t\"ByUser\":{\r\n\t\t\"byChannel\":\"#BYCHANNEL\",\r\n\t\t\"byUser\":\"#BYUSER\"\r\n\t},\r\n\t\"Customer\":{\r\n\t\t\"CustomerID\":#CUSTOMERID\r\n\t},\r\n\t\"Product\":{\r\n\t\t\"Product\":[{\r\n\t\t\t\"ProductId\":#PRODUCTID\t\t\r\n\t\t}]\r\n\t},\r\n\t\"Reason\":#REASON,\r\n\t\"Target\":{\r\n\t\t\"Target\":#TARGET,\r\n\t\t\"TargetDate\":\"\"\r\n\t}\r\n}"
			strReq = strings.Replace(strReq, "#BYCHANNEL", "", -1)
			strReq = strings.Replace(strReq, "#BYUSER", "BILL-COL", -1)
			strReq = strings.Replace(strReq, "#CUSTOMERID", req.TVSCustomer, -1)
			strReq = strings.Replace(strReq, "#PRODUCTID", strconv.Itoa(productID), -1)
			strReq = strings.Replace(strReq, "#REASON", strconv.Itoa(reason), -1)
			strReq = strings.Replace(strReq, "#TARGET", "0", -1)

			req := []byte(strReq)

			response, err := http.Post(cfg.disconnectURL, "application/json", bytes.NewBuffer(req))
			if err != nil {
				fmt.Printf("The HTTP request failed with error %s", err)
			} else {
				data, _ := ioutil.ReadAll(response.Body)
				//fmt.Println(string(data))
				//myReturn = json.Unmarshal(string(data))
				err = json.Unmarshal(data, &serviceRes)
				if err != nil {
					//panic(err)
					fmt.Printf("The HTTP response failed with error %s", err)
				} else {
					fmt.Printf("## Result >> %d %d %s", serviceRes.ProductID, serviceRes.ErrorCode, serviceRes.ErrorDesc)
					result.Status = serviceRes.ResultValue
					result.ErrorCode = serviceRes.ErrorCode
					result.ErrorDescription = serviceRes.ErrorDesc
				}
			}
		}

		if !haveProduct {
			result.Status = "true"
			result.ErrorCode = 0
			result.ErrorDescription = "No products to disconnectPTP"
		}
	}

	// Disconnect all child account
	var rsChild driver.Rows
	var childAccount int
	bResult = ExecuteStoreProcedure(cfg.dbICC, "begin PK_CCBS_COLLECTION.GetChildAccount(:1,:2,:3,:4); end;", req.TVSCustomer, req.ActionCode,
		req.ActivityReasonCode, sql.Out{Dest: &rsChild})
	if bResult && rsChild != nil {
		values := make([]driver.Value, len(rs.Columns()))
		log.Printf("## Process to disconnecct child account")

		for rsChild.Next(values) == nil {
			childAccount, _ = strconv.Atoi(strconv.FormatInt(values[0].(int64), 10))

			log.Printf("## Product Disconnect = %d %d %d", productID, commercialProductID, reason)

			// Call Service to disconnect
			var serviceRes ServiceResult

			strReq := "{\r\n\t\"ByUser\":{\r\n\t\t\"byChannel\":\"#BYCHANNEL\",\r\n\t\t\"byUser\":\"#BYUSER\"\r\n\t},\r\n\t\"Customer\":{\r\n\t\t\"CustomerID\":#CUSTOMERID\r\n\t},\r\n\t\"Reason\":#REASON,\r\n\t\"Target\":{\r\n\t\t\"Target\":#TARGET,\r\n\t\t\"TargetDate\":\"\"\r\n\t}\r\n}"
			strReq = strings.Replace(strReq, "#BYCHANNEL", "", -1)
			strReq = strings.Replace(strReq, "#BYUSER", "BILL-COL", -1)
			strReq = strings.Replace(strReq, "#CUSTOMERID", strconv.Itoa(childAccount), -1)
			strReq = strings.Replace(strReq, "#REASON", strconv.Itoa(reason), -1)
			strReq = strings.Replace(strReq, "#TARGET", "0", -1)

			req := []byte(strReq)

			response, err := http.Post(cfg.disconnectURL, "application/json", bytes.NewBuffer(req))
			if err != nil {
				log.Printf("The HTTP request failed with error %s", err)
			} else {
				data, _ := ioutil.ReadAll(response.Body)
				//fmt.Println(string(data))
				//myReturn = json.Unmarshal(string(data))
				err = json.Unmarshal(data, &serviceRes)
				if err != nil {
					//panic(err)
					log.Printf("The HTTP response failed with error %s", err)
				} else {
					log.Printf("## Result >> %d %d %s", serviceRes.ProductID, serviceRes.ErrorCode, serviceRes.ErrorDesc)
					result.Status = serviceRes.ResultValue
					result.ErrorCode = serviceRes.ErrorCode
					result.ErrorDescription = serviceRes.ErrorDesc
				}
			}
			if childAccount == 0 {
				log.Printf("## No account child to disconnect")
			}
		}
	}
	// End Disconnect all child

	return result, nil
}

func isCMDUCustomer(tvsCustomer string) int {
	isCMDU := 0
	aSQL := "select pk_ccbs_collection.IsCMDUCustomer(" + tvsCustomer + ") allow from dual"
	rows, err := SelectSQL(cfg.dbICC, aSQL)
	// close database connection after this main function finished
	defer rows.Close()
	if err != nil {
		// error
		isCMDU = 0
	} else {
		if rows.Next() {
			rows.Scan(&isCMDU)
		}
	}
	return isCMDU
}

func getChildAccountCount(tvsCustomer string) int {
	cntChild := 0
	aSQL := "select pk_ccbs_collection.GetChildAccountCount(" + tvsCustomer + ") cnt from dual"

	rows, err := SelectSQL(cfg.dbICC, aSQL)
	// close database connection after this main function finished
	defer rows.Close()
	if err != nil {
		// error
		cntChild = 0
	} else {
		if rows.Next() {
			rows.Scan(&cntChild)
		}
	}

	//log.Printf("Child Cnt = %d", cntChild)

	return cntChild
}

func getCCBSAccountByTVSCustomer(tvsCustomer string) int {
	ccbsAccount := 0
	aSQL := "select PK_CCBS_ICCSERVICE.GetCCBSAccountByTVSCustomer(" + tvsCustomer + ") ccbsacct from dual"
	rows, err := SelectSQL(cfg.dbICC, aSQL)
	// close database connection after this main function finished
	defer rows.Close()
	if err != nil {
		// error
		ccbsAccount = 0
	} else {
		if rows.Next() {
			rows.Scan(&ccbsAccount)
		}
	}

	log.Printf("CCBS Account = %d", ccbsAccount)

	return ccbsAccount
}

func getCCBSAccountBalance(ccbsAccount int) float64 {
	var acctBalance float64
	url := fmt.Sprintf("%s", cfg.ccbsAccountURL)

	payloadStr := strings.TrimSpace(`
    <soapenv:Envelope
       xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"
	   xmlns:tem="http://tempuri.org/"
	   xmlns:tvs="http://schemas.datacontract.org/2004/07/TVSPayment">	   
		<soapenv:Body>
		<tem:GetAccountBalance>    
		  	<tem:inReq>        
				<tvs:TransInfo>		   
		   		<tvs:Channel>TVS</tvs:Channel>		   
		   		<tvs:TransId>1</tvs:TransId>		
			   	<tvs:UserId>1</tvs:UserId>
			</tvs:TransInfo>		
			<tvs:AccountId>:TVSCustomer</tvs:AccountId>				   
	 		</tem:inReq>
		</tem:GetAccountBalance>
    </soapenv:Body>
	</soapenv:Envelope>`)
	payloadStr = strings.Replace(payloadStr, ":TVSCustomer", strconv.Itoa(ccbsAccount), -1)

	payload := []byte(payloadStr)

	log.Printf("%s", url)
	log.Printf("%s", payload)

	soapAction := "http://tempuri.org/IAccountService/GetAccountBalance"
	httpMethod := "POST"

	req, err := http.NewRequest(httpMethod, url, bytes.NewReader(payload))
	if err != nil {
		log.Fatal("Error on creating request object. ", err.Error())
		//return
	}

	req.Header.Set("Content-type", "text/xml")
	req.Header.Set("SOAPAction", soapAction)
	req.Header.Set("Accept", "text/xml")

	client := &http.Client{}

	res, err := client.Do(req)
	if err != nil {
		log.Fatal("Error on dispatching request. ", err.Error())
		//return
	}

	result := new(GetBalance)
	err = xml.NewDecoder(res.Body).Decode(result)
	if err != nil {
		log.Fatal("Error on unmarshaling xml. ", err.Error())
		//return
	} else {
		acctBalance, _ = strconv.ParseFloat(result.Body.GetAccountBalanceResponse.GetAccountBalanceResult.SearchResult.ArBalanceField.Text, 64)
	}

	return acctBalance
}

// Reconnect Service
func Reconnect(req OrderRequest, reconType string) (OrderResponse, error) {
	var result OrderResponse

	log.Printf("## Process " + reconType + " Service")

	isAllow := 1
	// ############# Verify Profile to reconnect
	// ## check cmdu
	isCMDU := isCMDUCustomer(req.TVSCustomer)
	if isCMDU == 1 {
		// TODO - Sent Event 133 for CMDU

		isAllow = 0
	}
	// ## end check cmdu

	// ## get child account
	childCnt := getChildAccountCount(req.TVSCustomer)
	if childCnt > 0 {
		// TODO - Sent Event 133 for parent

		// TODO - Sent Event 133 for all child account
		var rsChild driver.Rows
		var reconSQL string
		if reconType == "Reconnect" {
			reconSQL = "begin PK_CCBS_COLLECTION.GetReconAgreementDetail(:1,:2,:3,:4); end;"
		} else {
			reconSQL = "begin PK_CCBS_COLLECTION.GetReconPTPAgreementDetail(:1,:2,:3,:4); end;"
		}
		bResult := ExecuteStoreProcedure(cfg.dbICC, reconSQL, req.TVSCustomer, req.ActionCode,
			req.ActivityReasonCode, sql.Out{Dest: &rsChild})
		if bResult && rsChild != nil {
			values := make([]driver.Value, len(rsChild.Columns()))
			for rsChild.Next(values) == nil {
				// TODO - Sent Event 133 for all child account

			}
		}
		isAllow = 0
	}
	// ## end get child account

	// ## SPActivityReason for reason 473 >> MANRS
	var ccbsBalance float64
	spReason := 0
	isPaymentNoReconFee := 0
	if req.ActivityReasonCode == "MANRS" {
		spReason = 473
	} else {
		ccbsAccount := getCCBSAccountByTVSCustomer(req.TVSCustomer)
		ccbsBalance := getCCBSAccountBalance(ccbsAccount)

		isPaymentNoReconFee = 1
		isAllow = 0
		_ = ccbsBalance
	}

	if isAllow == 1 {
		// get product to Reconnect
		var rs driver.Rows
		var productID int
		var commercialProductID int
		var profile string
		var reason int
		//var reconfeeAmt goracle.Number
		var reconSQL string
		if reconType == "Reconnect" {
			reconSQL = "begin PK_CCBS_COLLECTION.GetReconAgreementDetail(:1,:2,:3,:4); end;"
		} else {
			reconSQL = "begin PK_CCBS_COLLECTION.GetReconPTPAgreementDetail(:1,:2,:3,:4); end;"
		}
		bResult := ExecuteStoreProcedure(cfg.dbICC, reconSQL, req.TVSCustomer, req.ActionCode,
			req.ActivityReasonCode, sql.Out{Dest: &rs})
		if bResult && rs != nil {
			values := make([]driver.Value, len(rs.Columns()))
			// ok
			haveProduct := false
			for rs.Next(values) == nil {

				haveProduct = true

				productID, _ = strconv.Atoi(strconv.FormatInt(values[0].(int64), 10))
				commercialProductID, _ = strconv.Atoi(strconv.FormatInt(values[1].(int64), 10))
				profile = values[2].(string)
				reason, _ = strconv.Atoi(strconv.FormatInt(values[3].(int64), 10))
				//for special activity reason >> 473 recon dnp except recon fee
				if spReason != 0 {
					reason = 473
				}
				reconfeeAmt := values[4].(float64)
				if ccbsBalance > reconfeeAmt {
					// TODO - Sent Event 133 for all child account

					break //end loop
				}

				log.Printf("## Product %s = %d %d %s %d %f", reconType, productID, commercialProductID, profile, reason, reconfeeAmt)

				// Call Service to Reconnect
				var serviceRes ServiceResult

				strReq := "{\r\n\t\"ByUser\":{\r\n\t\t\"byChannel\":\"#BYCHANNEL\",\r\n\t\t\"byUser\":\"#BYUSER\"\r\n\t},\r\n\t\"Customer\":{\r\n\t\t\"CustomerID\":#CUSTOMERID\r\n\t},\r\n\t\"Product\":{\r\n\t\t\"Product\":[{\r\n\t\t\t\"ProductId\":#PRODUCTID\t\t\r\n\t\t}]\r\n\t},\r\n\t\"Reason\":#REASON,\r\n\t\"Target\":{\r\n\t\t\"Target\":#TARGET,\r\n\t\t\"TargetDate\":\"\"\r\n\t}\r\n}"
				strReq = strings.Replace(strReq, "#BYCHANNEL", "", -1)
				strReq = strings.Replace(strReq, "#BYUSER", "BILL-COL", -1)
				strReq = strings.Replace(strReq, "#CUSTOMERID", req.TVSCustomer, -1)
				strReq = strings.Replace(strReq, "#PRODUCTID", strconv.Itoa(productID), -1)
				strReq = strings.Replace(strReq, "#REASON", strconv.Itoa(reason), -1)
				strReq = strings.Replace(strReq, "#TARGET", "0", -1)

				req := []byte(strReq)

				response, err := http.Post(cfg.reconnectURL, "application/json", bytes.NewBuffer(req))
				if err != nil {
					log.Printf("The HTTP request failed with error %s", err)
				} else {
					data, _ := ioutil.ReadAll(response.Body)
					//fmt.Println(string(data))
					//myReturn = json.Unmarshal(string(data))
					err = json.Unmarshal(data, &serviceRes)
					if err != nil {
						//panic(err)
						log.Printf("The HTTP response failed with error %s", err)
					} else {
						log.Printf("## Result >> %d %d %s", serviceRes.ProductID, serviceRes.ErrorCode, serviceRes.ErrorDesc)
						result.Status = serviceRes.ResultValue
						result.ErrorCode = serviceRes.ErrorCode
						result.ErrorDescription = serviceRes.ErrorDesc
					}
				}
			}

			if !haveProduct {
				result.Status = "true"
				result.ErrorCode = 0
				result.ErrorDescription = "No products to reconnect"
			}
		}
	} else {
		//### do something
		// todo

		if isCMDU == 1 {
			result.Status = "true"
			result.ErrorCode = 0
			result.ErrorDescription = "This tvs profile is CMDU customer"
		} else if isPaymentNoReconFee == 1 {
			result.Status = "true"
			result.ErrorCode = 0
			result.ErrorDescription = "Payment not cover recon fee"
		} else {
			result.Status = "true"
			result.ErrorCode = 0
			result.ErrorDescription = "This customer not allow to reconnect"
		}

	}

	return result, nil
}

// ReconnectPTP Service
/*func ReconnectPTP(req OrderRequest) (OrderResponse, error) {
	var result OrderResponse

	log.Printf("## Process ReconnectPTP Service")

	// get product to ReconnectPTP
	var rs driver.Rows
	var productID int
	var commercialProductID int
	var profile string
	var reason int
	var reconfeeAmt goracle.Number
	bResult := ExecuteStoreProcedure(cfg.dbICC, "begin PK_CCBS_COLLECTION.GetReconPTPAgreementDetail(:1,:2,:3,:4); end;", req.TVSCustomer, req.ActionCode,
		req.ActivityReasonCode, sql.Out{Dest: &rs})
	if bResult && rs != nil {
		values := make([]driver.Value, len(rs.Columns()))
		// ok
		haveProduct := false
		for rs.Next(values) == nil {
			haveProduct = true

			productID, _ = strconv.Atoi(strconv.FormatInt(values[0].(int64), 10))
			commercialProductID, _ = strconv.Atoi(strconv.FormatInt(values[1].(int64), 10))
			profile = values[2].(string)
			reason, _ = strconv.Atoi(strconv.FormatInt(values[3].(int64), 10))
			reconfeeAmt = values[4].(goracle.Number)

			//strconv.Atoi(strconv.FormatInt(productID, 10))
			//_ = commercialProductID
			log.Printf("## Product ReconnectPTP = %d %d %s %d %s", productID, commercialProductID, profile, reason, reconfeeAmt)

			// Call Service to Reconnect
			//jsonData := map[string]string{"ThaiId": "3909800183384"}
			//var serviceReq ServiceRequest
			var serviceRes ServiceResult

			strReq := "{\r\n\t\"ByUser\":{\r\n\t\t\"byChannel\":\"#BYCHANNEL\",\r\n\t\t\"byUser\":\"#BYUSER\"\r\n\t},\r\n\t\"Customer\":{\r\n\t\t\"CustomerID\":#CUSTOMERID\r\n\t},\r\n\t\"Product\":{\r\n\t\t\"Product\":[{\r\n\t\t\t\"ProductId\":#PRODUCTID\t\t\r\n\t\t}]\r\n\t},\r\n\t\"Reason\":#REASON,\r\n\t\"Target\":{\r\n\t\t\"Target\":#TARGET,\r\n\t\t\"TargetDate\":\"\"\r\n\t}\r\n}"
			strReq = strings.Replace(strReq, "#BYCHANNEL", "", -1)
			strReq = strings.Replace(strReq, "#BYUSER", "BILL-COL", -1)
			strReq = strings.Replace(strReq, "#CUSTOMERID", req.TVSCustomer, -1)
			strReq = strings.Replace(strReq, "#PRODUCTID", strconv.Itoa(productID), -1)
			strReq = strings.Replace(strReq, "#REASON", strconv.Itoa(reason), -1)
			strReq = strings.Replace(strReq, "#TARGET", "0", -1)

			req := []byte(strReq)

			//fmt.Printf("%s\n", strReq)

			//json.Unmarshal(req, &serviceReq)
			response, err := http.Post(cfg.reconnectURL, "application/json", bytes.NewBuffer(req))
			if err != nil {
				fmt.Printf("The HTTP request failed with error %s", err)
			} else {
				data, _ := ioutil.ReadAll(response.Body)
				//fmt.Println(string(data))
				//myReturn = json.Unmarshal(string(data))
				err = json.Unmarshal(data, &serviceRes)
				if err != nil {
					//panic(err)
					fmt.Printf("The HTTP response failed with error %s", err)
				} else {
					fmt.Printf("## Result %d %d %s", serviceRes.ProductID, serviceRes.ErrorCode, serviceRes.ErrorDesc)
					result.Status = serviceRes.ResultValue
					result.ErrorCode = serviceRes.ErrorCode
					result.ErrorDescription = serviceRes.ErrorDesc
				}
			}
		}

		if !haveProduct {
			result.Status = "true"
			result.ErrorCode = 0
			result.ErrorDescription = "No products to reconnectPTP"
		}
	}

	return result, nil
}*/
