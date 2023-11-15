package goupnp

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

type reqAction struct {
	name  string
	space string
	inner interface{}
}

func (a reqAction) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	return e.EncodeElement(a.inner, xml.StartElement{
		Name: xml.Name{Local: a.name},
		Attr: []xml.Attr{{Name: xml.Name{Local: "xmlns:u"}, Value: a.space}},
	})
}

type reqEnvelope struct {
	XMLName       xml.Name `xml:"s:Envelope"`
	Space         string   `xml:"xmlns:s,attr"`
	EncodingStyle string   `xml:"s:encodingStyle,attr"`
	Body          struct {
		XMLName xml.Name `xml:"s:Body"`
		Action  reqAction
	}
}

type respEnvelope struct {
	XMLName       xml.Name `xml:"http://schemas.xmlsoap.org/soap/envelope/ Envelope"`
	EncodingStyle string   `xml:"http://schemas.xmlsoap.org/soap/envelope/ encodingStyle,attr"`
	Body          struct {
		Fault *struct {
			FaultCode   string `xml:"faultcode"`
			FaultString string `xml:"faultstring"`
			Detail      *struct {
				UPnPError *struct {
					Code        string `xml:"errorCode"`
					Description string `xml:"errorDescription"`
				} `xml:"UPnPError"`
			} `xml:"detail"`
		} `xml:"Fault"`
		RawAction []byte `xml:",innerxml"`
	} `xml:"http://schemas.xmlsoap.org/soap/envelope/ Body"`
}

func encodeRequest(actionNamespace string, actionName string, action interface{}) string {
	e := reqEnvelope{
		Space:         "http://schemas.xmlsoap.org/soap/envelope/",
		EncodingStyle: "http://schemas.xmlsoap.org/soap/encoding/",
	}
	e.Body.Action = reqAction{
		name:  "u:" + actionName,
		space: actionNamespace,
		inner: action,
	}
	b, _ := xml.Marshal(e)
	return xml.Header + string(b)
}

func performSOAPAction(url string, actionNamespace, actionName string, req interface{}, resp interface{}) error {
	requestBody := strings.NewReader(encodeRequest(actionNamespace, actionName, req))
	httpReq, _ := http.NewRequest("POST", url, requestBody)
	httpReq.Header.Set("SOAPACTION", fmt.Sprintf(`"%s#%s"`, actionNamespace, actionName))
	httpReq.Header.Set("CONTENT-TYPE", `text/xml; charset="utf-8"`)
	httpReq.ContentLength = int64(requestBody.Len())
	response, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	defer ioutil.ReadAll(response.Body)

	var responseEnv respEnvelope
	decoder := xml.NewDecoder(response.Body)
	if err := decoder.Decode(&responseEnv); err != nil {
		return fmt.Errorf("invalid response body: %w", err)
	} else if f := responseEnv.Body.Fault; f != nil {
		if f.Detail == nil || f.Detail.UPnPError == nil {
			return fmt.Errorf("SOAP fault: %s", responseEnv.Body.Fault.FaultString)
		}
		e := f.Detail.UPnPError
		return fmt.Errorf("UPnP error: %s (error code %s)", e.Description, e.Code)
	}
	if resp != nil {
		if err := xml.Unmarshal(responseEnv.Body.RawAction, resp); err != nil {
			return fmt.Errorf("invalid response body: %w", err)
		}
	}

	return nil
}
