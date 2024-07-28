package nupf

// UPF Event
type FlowSampling struct {
	UplinkTimeSampling    string `json:"uplinkTimeSampling"`
	UplinkCountSampling   string `json:"uplinkCountSampling"`
	DownlinkTimeSampling  string `json:"downlinkTimeSampling"`
	DownlinkCountSampling string `json:"downlinkCountSampling"`
}
