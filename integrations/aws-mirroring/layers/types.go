package layers

type Channels struct {
	LogsChan           chan *SuricataHTTPEvent
	OtelAttributesChan chan *OTELAttributes
}

type SuricataHTTPEvent struct {
	Request  *HTTPRequest  `json:"request"`
	Response *HTTPResponse `json:"response"`
	Metadata *HTTPMetadata `json:"metadata"`
}

type HTTPMetadata struct {
	Timestamp string `json:"timestamp"`
	SrcPort   int    `json:"src_port"`
	SrcIP     string `json:"src_ip"`
	DestPort  int    `json:"dest_port"`
	DestIP    string `json:"dest_ip"`
}

type HTTPRequest struct {
	Header map[string]interface{} `json:"header"`
	Body   string                 `json:"body"`
}

type HTTPResponse struct {
	Header map[string]interface{} `json:"header"`
	Body   string                 `json:"body"`
}

type OTELAttributes struct {
	HTTPMethod      string
	HTTPFlavor      string
	HTTPTarget      string
	HTTPHost        string
	HTTPStatusCode  int
	HTTPScheme      string
	NetHostPort     int
	NetPeerIP       string
	NetPeerPort     int
	SensorVersion   string
	SensorID        string
	RequestBody     string
	RequestHeaders  string
	ResponseHeaders string
	ResponseBody    string
}
