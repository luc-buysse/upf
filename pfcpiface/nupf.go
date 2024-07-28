package pfcpiface

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/gorilla/mux"
	"github.com/omec-project/upf-epc/internal/p4constants"
	nupf "github.com/omec-project/upf-epc/pfcpiface/nupf"
	p4 "github.com/p4lang/p4runtime/go/p4/v1"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
)

type Route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc http.HandlerFunc
}

type reportInfo struct {
	ipAddr           net.IP
	correlationId    string
	notificationAddr string
	period           uint32
	trendMonitor     *trendMonitor

	subscriptionId string

	events []nupf.UpfEvent
	last   map[string]p4.CounterData

	ctrs map[string][]uint32
}

type intReportInfo struct {
	ipAddr           net.IP
	correlationId    string
	notificationAddr string

	subscriptionId string
}

type trendMonitorBuffer struct {
	UlAverageThroughput       float64
	DlAverageThroughput       float64
	UlPeakThroughput          float64
	DlPeakThroughput          float64
	UlAveragePacketThroughput float64
	DlAveragePacketThroughput float64
	UlPeakPacketThroughput    float64
	DlPeakPacketThroughput    float64
}

type trendMonitorAcc struct {
	UlVolume      uint64
	DlVolume      uint64
	UlNbOfPackets uint64
	DlNbOfPackets uint64
}

type trendMonitor struct {
	tmp trendMonitorBuffer
	acc trendMonitorAcc
}

type Nupf struct {
	Subscriptions map[string][]byte
	BaseUri       string
	NextID        int
	Mutex         *sync.Mutex
	Client        *http.Client
	InstanceID    string
	TrendMutex    sync.Mutex

	ReportInfos        map[uint32][]*reportInfo
	ReportMutexes      map[uint32]*sync.Mutex
	IntReportInfos     map[uint32]*intReportInfo
	TrendMonitors      map[uint32]*trendMonitor
	ReportInfosById    map[string]*reportInfo
	IntReportInfosById map[string]*intReportInfo
	ReportMutexesById  map[string]*sync.Mutex

	app *PFCPIface
}

type INTReportMetadata struct {
	Ver              int  `json:"ver"`
	NProto           int  `json:"NProto"`
	D                bool `json:"D"`
	Q                bool `json:"Q"`
	F                bool `json:"F"`
	Hw_id            int  `json:"hw_id"`
	Seq_no           int  `json:"seq_no"`
	Ig_tstamp        int  `json:"ig_tstamp"`
	Switch_id        int  `json:"switch_id"`
	Ingress_port_id  int  `json:"ingress_port_id"`
	Egress_port_id   int  `json:"egress_port_id"`
	Queue_id         int  `json:"queue_id"`
	Queue_occupancy  int  `json:"queue_occupancy"`
	Egress_timestamp int  `json:"egress_timestamp"`
	Pkt_count        int  `json:"pkt_count"`
	Byte_count       int  `json:"byte_count"`
}

type INTReportEth struct {
	Dst_mac   string `json:"dst_mac"`
	Src_mac   string `json:"src_mac"`
	Eth_proto int    `json:"eth_proto"`
}

type INTReportIP struct {
	Version int    `json:"version"`
	Ihl     int    `json:"ihl"`
	Ttl     int    `json:"ttl"`
	Proto   int    `json:"proto"`
	Src_ip  net.IP `json:"src_ip"`
	Dest_ip net.IP `json:"dest_ip"`
	Len     int    `json:"len"`
}

type INTReportTcp struct {
	Sport   int `json:"sport"`
	Dport   int `json:"dport"`
	Seq     int `json:"seq"`
	Ack     int `json:"ack"`
	Dataofs int `json:"dataofs"`
	Flags   int `json:"flags"`
	Window  int `json:"window"`
	Urgptr  int `json:"urgptr"`
}

type INTReportICMP struct {
	Typ  int `json:"typ"`
	Code int `json:"code"`
	Id   int `json:"id"`
	Seq  int `json:"seq"`
}

type INTReportUdp struct {
	Sport int `json:"sport"`
	Dport int `json:"dport"`
	Len   int `json:"len"`
	Seq   int `json:"seq"`
}

type INTReport struct {
	Meta INTReportMetadata `json:"meta"`
	Eth  INTReportEth      `json:"eth"`
	Ipv4 INTReportIP       `json:"ipv4"`
	Tcp  *INTReportTcp     `json:"tcp",omitempty`
	Icmp *INTReportICMP    `json:"icmp",omitempty`
	Udp  *INTReportUdp     `json:"udp",omitempty`
}

type httpProblem struct {
	Typ      string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail"`
	Instance string `json:"instance"`
}

type subscriptionError struct {
	value string
}

func (e *subscriptionError) Error() string {
	return e.value
}

func NewSubscriptionError(v string) (e *subscriptionError) {
	return &subscriptionError{
		value: v,
	}
}

// GET /
func Index(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello World!")
}

// DELETE /ee-Subscriptions/{subscriptionId}
func (u *Nupf) DeleteSubscriptionHandle(w http.ResponseWriter, r *http.Request) {
	err := u.DeleteSubscription(mux.Vars(r)["subscriptionId"])
	if err != nil {
		log.Errorln("Failed to delete a subscription")

		responseContentRaw := httpProblem{
			Typ:      r.Host + r.RequestURI,
			Title:    "Unable to delete the subscription",
			Status:   500,
			Detail:   err.Error(),
			Instance: r.RequestURI,
		}

		responseContent, err := json.Marshal(responseContentRaw)
		if err != nil {
			log.Errorln("Failed to marshal the error response: ", err)
		}

		w.Header().Set("Content-Type", "application/problem+json; charset=UTF-8")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(responseContent)
	} else {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusNoContent)
	}
}

// PATCH /ee-Subscriptions/{subscriptionId}
func (u *Nupf) ModifySubscriptionHandle(w http.ResponseWriter, r *http.Request) {
	var (
		err error
	)

	result, err := u.ModifySubscription(r)
	if err != nil {
		log.Errorln("Failed to modify a subscription")

		responseContentRaw := httpProblem{
			Typ:      r.Host + r.RequestURI,
			Title:    "Unable to modify the subscription",
			Status:   500,
			Detail:   err.Error(),
			Instance: r.RequestURI,
		}

		responseContent, err := json.Marshal(responseContentRaw)
		if err != nil {
			log.Errorln("Failed to marshal the error response: ", err)
		}

		w.Header().Set("Content-Type", "application/problem+json; charset=UTF-8")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(responseContent)
	} else {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusNoContent)
		w.Write(result)
	}
}

// POST /ee-Subscriptions
func (u *Nupf) CreateSubscriptionHandle(w http.ResponseWriter, r *http.Request) {
	var (
		err error
	)

	result, err := u.CreateSubscription(r)
	if err != nil {
		log.Errorln("Failed to create a new subscription: ", err)

		responseContentRaw := httpProblem{
			Typ:      r.Host + r.RequestURI,
			Title:    "Unable to create new subscription",
			Status:   500,
			Detail:   err.Error(),
			Instance: r.RequestURI,
		}

		responseContent, err := json.Marshal(responseContentRaw)
		if err != nil {
			log.Errorln("Failed to marshal the error response: ", err)
		}

		w.Header().Set("Content-Type", "application/problem+json; charset=UTF-8")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(responseContent)
	} else {
		reponseContent, err := json.Marshal(result)
		if err != nil {
			log.Errorln("Cannot marshal the create subscription response")
		}

		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusCreated)
		w.Write(reponseContent)
	}
}

func (u *Nupf) ModifySubscription(r *http.Request) ([]byte, error) {
	var (
		err error
	)

	subscriptionId := mux.Vars(r)["subscriptionId"]

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, NewSubscriptionError("Cannot extract the body of the create subscription request")
	}

	original, exists := u.Subscriptions[subscriptionId]
	if !exists {
		return nil, fmt.Errorf("no subscription was found with the provided id: %s", subscriptionId)
	}

	patched, err := jsonpatch.MergePatch(original, body)

	var req nupf.CreateEventSubscription
	if err = json.Unmarshal(patched, &req); err != nil {
		return nil, fmt.Errorf("cannot parse the body of create subscription request")
	}

	u.ProcessSubscription(*req.Subscription, subscriptionId, false)

	return patched, nil
}

func (u *Nupf) DeleteSubscription(subscriptionId string) error {

	mu, exists, err := u.LockSubscription(subscriptionId, false)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("subscription with id %s doesn't exist", subscriptionId)
	}
	defer mu.Unlock()
	delete(u.ReportMutexesById, subscriptionId)

	reportInfo, exists := u.ReportInfosById[subscriptionId]
	if exists {
		delete(u.ReportInfosById, subscriptionId)

		u.ReportMutexes[reportInfo.period].Lock()
		defer u.ReportMutexes[reportInfo.period].Unlock()
		sl := u.ReportInfos[reportInfo.period]
		if len(sl) == 1 {
			delete(u.ReportInfos, reportInfo.period)
			delete(u.ReportMutexes, reportInfo.period)
			return nil
		}

		for i, r := range sl {
			if r == reportInfo {
				copy(sl[i:], sl[i+1:])
				u.ReportInfos[reportInfo.period] = sl[:len(sl)-1]
			}
		}

		m, exists := u.TrendMonitors[ip2int(reportInfo.ipAddr)]
		if exists && m == reportInfo.trendMonitor {
			delete(u.TrendMonitors, ip2int(reportInfo.ipAddr))
		}

		return nil
	}

	intReportInfo, exists := u.IntReportInfosById[subscriptionId]
	if !exists {
		return fmt.Errorf("subscription with id %s doesn't exist", subscriptionId)
	}
	delete(u.IntReportInfosById, subscriptionId)
	delete(u.IntReportInfos, ip2int(intReportInfo.ipAddr))

	return nil
}

func (u *Nupf) ProcessSubscription(sub nupf.UpfEventSubscription, suscriptionID string, doCreate bool) error {
	var err error

	reportingMode := sub.EventReportingMode

	if reportingMode == nil {
		return fmt.Errorf("Providing a reporting mode is mandatory (PERIODIC, THRESHOLD, ONE_TIME)")
	}

	switch reportingMode.Trigger {
	case "PERIODIC":
		err = u.AddPeriodicReportsSubscription(sub, suscriptionID, doCreate)
		if err != nil {
			return err
		}
	case "THRESHOLD":
		err = u.AddContinuousReportSubscription(sub, suscriptionID, doCreate)
		if err != nil {
			return err
		}
	case "ONE_TIME":
		rep, err := u.CreateReportInfo(sub, "")
		if err != nil {
			return err
		}
		u.Report([]*reportInfo{rep}, true)
	default:
		return fmt.Errorf("unsupported reporting mode")
	}

	return nil
}

func (u *Nupf) NewSubscriptionId() string {
	u.Mutex.Lock()
	subscriptionID := fmt.Sprintf("%d", u.NextID)
	u.NextID += 1
	u.Mutex.Unlock()

	return subscriptionID
}

func (u *Nupf) CreateSubscription(r *http.Request) (*nupf.CreatedEventSubscription, error) {
	var (
		err error
	)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, NewSubscriptionError("Cannot extract the body of the create subscription request")
	}

	var req nupf.CreateEventSubscription
	if err = json.Unmarshal(body, &req); err != nil {
		return nil, NewSubscriptionError("Cannot parse the body of create subscription request")
	}

	subscriptionID := u.NewSubscriptionId()

	err = u.ProcessSubscription(*req.Subscription, subscriptionID, true)
	if err != nil {
		return nil, err
	}

	u.Subscriptions[subscriptionID] = body

	return &nupf.CreatedEventSubscription{
		Subscription:   req.Subscription,
		SubscriptionId: subscriptionID,
	}, nil
}

func (u *Nupf) INTReportHandle(w http.ResponseWriter, r *http.Request) {
	err := u.ProcessINTReport(r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

func (u *Nupf) ProcessINTReport(r *http.Request) error {
	var (
		err        error
		ok         bool
		reportInfo *intReportInfo
		isDl       bool
	)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Errorln("Failed to extract the body of the INT Report request")
		return err
	}

	var req INTReport
	if err = json.Unmarshal(body, &req); err != nil {
		log.Errorln("Failed to parse the INT Report request")
		return err
	}

	if req.Ipv4.Dest_ip == nil {
		return fmt.Errorf("received an INT packet without dest IP address: %s", req.Ipv4.Dest_ip)
	}

	trendMonitor, isTrendDl := u.TrendMonitors[ip2int(req.Ipv4.Dest_ip)]
	if isTrendDl {
		u.TrendMutex.Lock()
		trendMonitor.acc.DlNbOfPackets += uint64(req.Meta.Pkt_count)
		trendMonitor.acc.DlVolume += uint64(req.Meta.Byte_count)
		log.Infoln("New acc: ", trendMonitor.acc)
		u.TrendMutex.Unlock()
	} else {
		trendMonitor, isTrendUl := u.TrendMonitors[ip2int(req.Ipv4.Dest_ip)]
		if isTrendUl {
			u.TrendMutex.Lock()
			trendMonitor.acc.UlNbOfPackets += uint64(req.Meta.Pkt_count)
			trendMonitor.acc.UlVolume += uint64(req.Meta.Byte_count)
			log.Infoln("New acc: ", trendMonitor.acc)
			u.TrendMutex.Unlock()
		}
	}

	isDl = true
	reportInfo, isDl = u.IntReportInfos[ip2int(req.Ipv4.Dest_ip)]

	if !isDl {
		reportInfo, ok = u.IntReportInfos[ip2int(req.Ipv4.Src_ip)]
		if !ok {
			return fmt.Errorf("no report info found for messages between %s and %s", req.Ipv4.Src_ip, req.Ipv4.Dest_ip)
		}
	}

	var reqContentRaw nupf.NotificationData
	reqContentRaw.CorrelationId = reportInfo.correlationId
	reqContentRaw.NotificationItems = make([]nupf.NotificationItem, 1)

	var notifItem nupf.NotificationItem
	notifItem.EventType = "PER_FLOW_USAGE_MEASURES"
	if isDl {
		notifItem.UeIpv4Addr = req.Ipv4.Dest_ip
	} else {
		notifItem.UeIpv4Addr = req.Ipv4.Src_ip
	}
	notifItem.UserDataUsageMeasurements = make([]nupf.UserDataUsageMeasurements, 1)

	var measurement nupf.UserDataUsageMeasurements
	if isDl {
		measurement.FlowInfo = &nupf.FlowInformation{
			FlowDirection: "DOWNLINK",
		}
	} else {
		measurement.FlowInfo = &nupf.FlowInformation{
			FlowDirection: "UPLINK",
		}
	}

	measurement.VolumeMeasurement = &nupf.VolumeMeasurement{
		TotalNbOfPackets: uint64(req.Meta.Pkt_count),
		TotalVolume:      fmt.Sprintf("%dB", req.Meta.Byte_count),
	}

	stub := fmt.Sprintf("ipv4 %s %s", req.Ipv4.Src_ip, req.Ipv4.Dest_ip)
	if req.Tcp != nil {
		measurement.FlowInfo.FlowDescription = fmt.Sprintf("%s tcp %d %d", stub, req.Tcp.Sport, req.Tcp.Dport)
	} else if req.Udp != nil {
		measurement.FlowInfo.FlowDescription = fmt.Sprintf("%s udp %d %d", stub, req.Udp.Sport, req.Udp.Dport)
	} else if req.Icmp != nil {
		measurement.FlowInfo.FlowDescription = fmt.Sprintf("%s icmp", stub)
	}

	now := time.Now()
	notifItem.TimeStamp = &now
	notifItem.UserDataUsageMeasurements[0] = measurement
	reqContentRaw.NotificationItems[0] = notifItem

	reqContent, err := json.Marshal(reqContentRaw)
	if err != nil {
		return fmt.Errorf("impossible to marshal notification request")
	}

	httpReq, err := http.NewRequest("POST", fmt.Sprintf("http://%s/", reportInfo.notificationAddr), bytes.NewBuffer(reqContent))
	if err != nil {
		return fmt.Errorf("failed to create notification request to %s with error : %s", reportInfo.notificationAddr, err.Error())
	}

	httpReq.Header.Set("Content-Type", "application/json")

	response, err := u.Client.Do(httpReq)
	if err != nil || response.StatusCode != 204 {
		return fmt.Errorf("failed to send notification request to %s with error : %s", reportInfo.notificationAddr, err.Error())
	}

	return nil
}

func (u *Nupf) ReadData(ctrs []uint32) (*p4.CounterData, error) {
	var preqos p4.CounterData
	var postqos p4.CounterData
	for _, ctrIndexInt := range ctrs {
		cntrIndex := &p4.Index{Index: int64(ctrIndexInt)}

		cntrEntry := &p4.CounterEntry{
			CounterId: 0,
			Index:     cntrIndex,
		}

		ctrData, err := u.app.fp.(*UP4).p4client.ReadCounterEntry(cntrEntry)
		if err != nil {
			return nil, err
		}

		for _, entity := range ctrData.GetEntities() {
			counterEntry := entity.GetCounterEntry()
			counterData := counterEntry.GetData()
			switch counterEntry.GetCounterId() {
			case p4constants.CounterPreQosPipePreQosCounter:
				preqos.ByteCount += counterData.GetByteCount()
				preqos.PacketCount += counterData.GetPacketCount()
			case p4constants.CounterPostQosPipePostQosCounter:
				postqos.ByteCount += counterData.GetByteCount()
				postqos.PacketCount += counterData.GetPacketCount()
			}
		}
	}

	return &preqos, nil
}

func (u *Nupf) Report(reportInfos []*reportInfo, oneTime bool) error {
	notifyReqs := make(map[string]*nupf.NotificationData)
	notifyAddr := make(map[string]string)

	for _, reportInfo := range reportInfos {
		mu, exists, err := u.LockSubscription(reportInfo.subscriptionId, false)
		if !exists || err != nil {
			continue
		}

		connID := fmt.Sprintf("%s|%s", reportInfo.notificationAddr, reportInfo.correlationId)
		req, ok := notifyReqs[connID]
		if !ok {
			req = &nupf.NotificationData{
				CorrelationId: reportInfo.correlationId,
			}
			notifyReqs[connID] = req
			notifyAddr[connID] = reportInfo.notificationAddr
		}

		now := time.Now()
		notificationItem := nupf.NotificationItem{
			TimeStamp:                 &now,
			EventType:                 "USER_DATA_USAGE_MEASURES",
			UserDataUsageMeasurements: make([]nupf.UserDataUsageMeasurements, 0),
		}
		for _, event := range reportInfo.events {
			if event.GranularityOfMeasurement != "PER_SESSION" {
				// TODO: Implement PER_FLOW and PER_APPLICATION measurement
				continue
			}

			switch event.Type_ {
			case "USER_DATA_USAGE_MEASURES":
				uplinkData, err := u.ReadData(reportInfo.ctrs["uplink"])
				if err != nil {
					log.Errorln("Cannot read uplink data: counter: ", reportInfo.ctrs["uplink"], " error: ", err)
					continue
				}

				downlinkData, err := u.ReadData(reportInfo.ctrs["downlink"])
				if err != nil {
					log.Errorln("Cannot read downlink data: counter: ", reportInfo.ctrs["downlink"], " error: ", err)
					continue
				}

				var measurements nupf.UserDataUsageMeasurements

				if oneTime {
					reportInfo.last["uplink"] = p4.CounterData{}
					reportInfo.last["downlink"] = p4.CounterData{}
				}

				lastDl, exists := reportInfo.last["uplink"]
				if exists {
					lastUl := reportInfo.last["downlink"]

					var diffUl p4.CounterData
					diffUl.ByteCount = uplinkData.ByteCount - lastUl.ByteCount
					diffUl.PacketCount = uplinkData.PacketCount - lastUl.PacketCount

					var diffDl p4.CounterData
					diffDl.ByteCount = downlinkData.ByteCount - lastDl.ByteCount
					diffDl.PacketCount = downlinkData.PacketCount - lastDl.PacketCount

					for _, measurementType := range event.MeasurementTypes {
						switch measurementType {
						case "VOLUME_MEASUREMENT":
							measurements.VolumeMeasurement = &nupf.VolumeMeasurement{
								TotalVolume:      fmt.Sprintf("%dB", diffUl.GetByteCount()+diffDl.GetByteCount()),
								DlVolume:         fmt.Sprintf("%dB", diffDl.GetByteCount()),
								UlVolume:         fmt.Sprintf("%dB", diffUl.GetByteCount()),
								TotalNbOfPackets: uint64(diffUl.GetPacketCount() + diffDl.GetPacketCount()),
								DlNbOfPackets:    uint64(diffDl.GetPacketCount()),
								UlNbOfPackets:    uint64(diffUl.GetPacketCount()),
							}
							break
						case "THROUGHPUT_MEASUREMENT":
							measurements.ThroughputMeasurement = &nupf.ThroughputMeasurement{
								UlThroughput:       fmt.Sprintf("%fbps", float64(diffUl.GetByteCount())/float64(reportInfo.period)),
								DlThroughput:       fmt.Sprintf("%fbps", float64(diffDl.GetByteCount())/float64(reportInfo.period)),
								UlPacketThroughput: fmt.Sprintf("%fpps", float64(diffUl.GetPacketCount())/float64(reportInfo.period)),
								DlPacketThroughput: fmt.Sprintf("%fpps", float64(diffDl.GetPacketCount())/float64(reportInfo.period)),
							}

							break
						}
					}
				}
				reportInfo.last["uplink"] = *uplinkData
				reportInfo.last["downlink"] = *downlinkData

				notificationItem.UserDataUsageMeasurements = []nupf.UserDataUsageMeasurements{measurements}
				break
			case "USER_DATA_USAGE_TRENDS":
				u.TrendMutex.Lock()
				tmp := reportInfo.trendMonitor.tmp
				reportInfo.trendMonitor.tmp = trendMonitorBuffer{}
				u.TrendMutex.Unlock()

				var measurements nupf.ThroughputStatisticsMeasurement

				measurements.DlAveragePacketThroughput = fmt.Sprintf("%dpps", uint64(tmp.DlAveragePacketThroughput))
				measurements.DlAverageThroughput = fmt.Sprintf("%dbps", uint64(tmp.DlAverageThroughput))
				measurements.DlPeakPacketThroughput = fmt.Sprintf("%dpps", uint64(tmp.DlPeakPacketThroughput))
				measurements.DlPeakThroughPut = fmt.Sprintf("%dbps", uint64(tmp.DlPeakThroughput))
				measurements.UlAveragePacketThroughput = fmt.Sprintf("%dpps", uint64(tmp.UlAveragePacketThroughput))
				measurements.UlAverageThroughput = fmt.Sprintf("%dbps", uint64(tmp.UlAverageThroughput))
				measurements.UlPeakPacketThroughput = fmt.Sprintf("%dpps", uint64(tmp.UlPeakPacketThroughput))
				measurements.UlPeakThroughput = fmt.Sprintf("%dbps", uint64(tmp.UlPeakThroughput))

				notificationItem.UserDataUsageMeasurements = []nupf.UserDataUsageMeasurements{nupf.UserDataUsageMeasurements{
					ThroughputStatisticsMeasurement: &measurements,
				}}
				break
			}
		}

		req.NotificationItems = append(req.NotificationItems, notificationItem)

		mu.Unlock()
	}

	for id, req := range notifyReqs {
		notificationAddr := notifyAddr[id]

		// Nothing to send
		if len(req.NotificationItems) == 0 {
			continue
		}

		jsonData, err := json.Marshal(req)
		if err != nil {
			log.Errorln("Failed to marshal json data")
			continue
		}

		httpReq, err := http.NewRequest("POST", fmt.Sprintf("http://%s/", notificationAddr), bytes.NewBuffer(jsonData))
		if err != nil {
			log.Errorln("Failed to create notification request to ", notificationAddr, " Error: ", err)
			continue
		}

		httpReq.Header.Set("Content-Type", "application/json")

		response, err := u.Client.Do(httpReq)
		if err != nil || response.StatusCode != 204 {
			log.Errorln("Failed to send notification to ", notificationAddr)
		}
	}

	return nil
}

func (u *Nupf) RunAtPeriod(period uint32) {
	for {
		time.Sleep(time.Duration(period) * time.Second)

		u.Report(u.ReportInfos[period], false)
	}
}

func (u *Nupf) MonitorTrends(period uint32) {
	float_period := float64(period) / 1000.
	for {
		time.Sleep(time.Duration(period) * time.Millisecond)

		u.TrendMutex.Lock()

		for _, v := range u.TrendMonitors {
			dlPacket := float64(v.acc.DlNbOfPackets) / float_period
			ulPacket := float64(v.acc.UlNbOfPackets) / float_period
			dlVolume := float64(v.acc.DlVolume) / float_period
			ulVolume := float64(v.acc.UlVolume) / float_period

			v.tmp.DlAveragePacketThroughput += dlPacket
			v.tmp.UlAveragePacketThroughput += ulPacket
			v.tmp.DlAverageThroughput += dlVolume
			v.tmp.UlAverageThroughput += ulVolume

			v.tmp.DlPeakPacketThroughput = max(v.tmp.DlPeakPacketThroughput, dlPacket)
			v.tmp.UlPeakPacketThroughput = max(v.tmp.UlPeakPacketThroughput, ulPacket)
			v.tmp.DlPeakThroughput = max(v.tmp.DlPeakThroughput, dlVolume)
			v.tmp.UlPeakThroughput = max(v.tmp.UlPeakThroughput, ulVolume)

			v.acc = trendMonitorAcc{}
		}

		u.TrendMutex.Unlock()
	}
}

func (u *Nupf) AddPeriodicReportsSubscription(sub nupf.UpfEventSubscription, subscriptionId string, doCreate bool) error {
	period := sub.EventReportingMode.RepPeriod

	_, first := u.ReportInfos[period]
	if !first {
		u.ReportInfos[period] = make([]*reportInfo, 0)
		u.ReportMutexes[period] = new(sync.Mutex)
	}

	mu, exists, err := u.LockSubscription(subscriptionId, doCreate)
	if err != nil {
		return err
	}
	defer mu.Unlock()

	rep, err := u.CreateReportInfo(sub, subscriptionId)
	if err != nil {
		return err
	}
	rep.subscriptionId = subscriptionId

	if !exists {
		u.ReportMutexes[period].Lock()
		u.ReportInfos[period] = append(u.ReportInfos[period], rep)
		u.ReportMutexes[period].Unlock()
		u.ReportInfosById[subscriptionId] = rep
	} else {
		*u.ReportInfosById[subscriptionId] = *rep
	}

	if !first {
		go u.RunAtPeriod(period)
	}

	return nil
}

func CountSamplingTranslate(v string) uint8 {
	switch v {
	case "NONE":
		return 0
	case "256":
		return 1
	case "64":
		return 2
	case "16":
		return 3
	case "4":
		return 4
	case "ALL":
		return 5
	default:
		return DefaultSampling().count
	}
}

func TimeSamplingTranslate(v string) uint8 {
	switch v {
	case "NEVER":
		return 0
	case "1M":
		return 1
	case "8S":
		return 2
	case "1S":
		return 3
	case "250MS":
		return 4
	case "67MS":
		return 5
	case "16MS":
		return 6
	case "4MS":
		return 7
	default:
		return DefaultSampling().time
	}
}

func (u *Nupf) LockSubscription(subscriptionId string, doCreate bool) (*sync.Mutex, bool, error) {
	mu, exists := u.ReportMutexesById[subscriptionId]
	if !exists && !doCreate {
		return nil, false, fmt.Errorf("the subscription with id %s doesn't exist", subscriptionId)
	} else if exists {
		mu.Lock()
	}

	_, exists = u.ReportMutexesById[subscriptionId]
	// If it has been deleted while waiting for the lock
	if !exists {
		if doCreate {
			mu = new(sync.Mutex)
			u.ReportMutexesById[subscriptionId] = mu
			mu.Lock()
		} else {
			return nil, false, fmt.Errorf("the subscription with id %s doesn't exist", subscriptionId)
		}
	}

	return mu, exists, nil
}

func (u *Nupf) AddContinuousReportSubscription(sub nupf.UpfEventSubscription, subscriptionId string, doCreate bool) error {
	var (
		err error
	)

	if sub.UeIpAddress.Ipv4Addr == nil {
		return fmt.Errorf("Unsupported type of IP Address")
	}

	if sub.EventReportingMode.FlowSampling == nil {
		return fmt.Errorf("")
	}

	all, err := u.GetSessionPdrs(sub.UeIpAddress.Ipv4Addr)
	if err != nil {
		return err
	}

	for i := range all.pdrs {
		pdr := &all.pdrs[i]
		if pdr.srcIface == access {
			pdr.sampling.count = CountSamplingTranslate(sub.EventReportingMode.FlowSampling.UplinkCountSampling)
			pdr.sampling.time = TimeSamplingTranslate(sub.EventReportingMode.FlowSampling.UplinkTimeSampling)
		} else if pdr.srcIface == core {
			pdr.sampling.count = CountSamplingTranslate(sub.EventReportingMode.FlowSampling.DownlinkCountSampling)
			pdr.sampling.time = TimeSamplingTranslate(sub.EventReportingMode.FlowSampling.DownlinkTimeSampling)
		}
	}

	err = u.UpdateINTSamplingRate(all.pdrs, *all)
	if err != nil {
		return err
	}

	mu, exists, err := u.LockSubscription(subscriptionId, doCreate)
	if err != nil {
		return err
	}
	defer mu.Unlock()

	rep := &intReportInfo{
		ipAddr:           sub.UeIpAddress.Ipv4Addr,
		correlationId:    sub.NotifyCorrelationId,
		notificationAddr: sub.EventNotifyUri,
		subscriptionId:   subscriptionId,
	}

	if !exists {
		u.IntReportInfosById[subscriptionId] = rep
		u.IntReportInfos[ip2int(sub.UeIpAddress.Ipv4Addr)] = rep
	} else {
		*u.IntReportInfosById[subscriptionId] = *rep
	}

	return nil
}

func (u *Nupf) UpdateINTSamplingRate(pdrs []pdr, all PacketForwardingRules) error {
	var (
		err error
		FAR far
	)

	entriesToApply := make([]*p4.TableEntry, 0)
	entriesToDelete := make([]*p4.TableEntry, 0)
	up4 := u.app.fp.(*UP4)

	for i := range pdrs {
		pdr := pdrs[i]
		if err = verifyPDR(pdr); err != nil {
			return err
		}

		pdrLog := log.WithFields(log.Fields{
			"pdr": pdr,
		})

		FAR, err = findRelatedFAR(pdr, all.fars)
		if err != nil {
			pdrLog.Warning("no related FAR for PDR found: ", err)
			return err
		}

		pdrLog = pdrLog.WithField("related FAR", FAR)
		pdrLog.Debug("Found related FAR for PDR")

		tunnelParameters := tunnelParams{
			tunnelIP4Src: ip2int(up4.accessIP.IP),
			tunnelIP4Dst: FAR.tunnelIP4Dst,
			tunnelPort:   FAR.tunnelPort,
		}

		tunnelPeerID, exists := up4.getGTPTunnelPeer(tunnelParameters)
		if !exists && FAR.tunnelTEID != 0 && FAR.dstIntf != 3 /* Exception for CP GTP */ {
			return ErrNotFoundWithParam("allocated GTP tunnel peer ID", "tunnel params", tunnelParameters)
		}

		var sessMeter = meter{meterTypeSession, 0, 0}
		if len(pdr.qerIDList) == 2 {
			// if 2 QERs are provided, the second one is Session QER
			sessMeter = up4.meters[meterID{
				qerID: pdr.qerIDList[1],
				fseid: pdr.fseID,
			}]
			pdrLog.Debug("Application meter found for PDR: ", sessMeter)
		} // else: if only 1 QER provided, set sessMeterIdx to 0, and use only per-app metering

		sessionEntry, err := up4.p4RtTranslator.BuildSessionsTableEntry(pdr, sessMeter, tunnelPeerID.id, FAR.Buffers())
		if err != nil {
			return ErrOperationFailedWithReason("build P4rt table entry for Sessions table", err.Error())
		}

		entriesToApply = append(entriesToApply, sessionEntry)

		if pdr.sessionEntry != nil {
			entriesToDelete = append(entriesToDelete, pdr.sessionEntry)
		}
		pdr.sessionEntry = sessionEntry
	}

	err = up4.p4client.ApplyTableEntries(p4.Update_DELETE, entriesToDelete...)
	if err != nil {
		return fmt.Errorf("failed to delete previous session table entiries for pdr")
	}

	err = up4.p4client.ApplyTableEntries(p4.Update_MODIFY, entriesToApply...)
	if err != nil {
		p4Error, ok := err.(*P4RuntimeError)
		if !ok {
			// not a P4Runtime error, returning err
			return ErrOperationFailedWithReason("applying table entries to UP4", err.Error())
		}

		for _, status := range p4Error.Get() {
			// ignore ALREADY_EXISTS or OK
			if status.GetCanonicalCode() == int32(codes.AlreadyExists) ||
				status.GetCanonicalCode() == int32(codes.OK) {
				continue
			}

			return ErrOperationFailedWithReason("applying table entries to UP4", p4Error.Error())
		}
	}

	return nil
}

func (u *Nupf) GetSessionPdrs(ip_addr net.IP) (*PacketForwardingRules, error) {
	up4 := u.app.fp.(*UP4)

	fseid, ok := up4.ueAddrToFSEID[ip2int(ip_addr)]
	if !ok {
		return nil, fmt.Errorf("Session associated with ip address %s cannot be found", ip_addr)
	}

	var result *PacketForwardingRules = nil
	var err error
	u.app.node.pConns.Range(func(key, value interface{}) bool {
		pConn, ok := value.(*PFCPConn)
		session, ok := pConn.store.GetSession(fseid)
		if !ok {
			return false
		}
		if result != nil {
			err = fmt.Errorf("Multiple session were found matching ip address %s and FSEID %s", ip_addr, fseid)
		}
		result = &session.PacketForwardingRules
		return false
	})
	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, fmt.Errorf("No session was found associated with ip address %s and FSEID %s", ip_addr, fseid)
	}
	return result, nil
}

func (u *Nupf) CreateReportInfo(sub nupf.UpfEventSubscription, subscriptionId string) (*reportInfo, error) {
	var (
		err error
	)

	if sub.UeIpAddress.Ipv4Addr == nil {
		return nil, fmt.Errorf("Unsupported type of IP Address")
	}

	report := reportInfo{
		period:           sub.EventReportingMode.RepPeriod,
		ipAddr:           sub.UeIpAddress.Ipv4Addr,
		correlationId:    sub.NotifyCorrelationId,
		notificationAddr: sub.EventNotifyUri,
		subscriptionId:   subscriptionId,
		ctrs:             make(map[string][]uint32),
		events:           sub.EventList,
		last:             make(map[string]p4.CounterData),
	}

	all, err := u.GetSessionPdrs(sub.UeIpAddress.Ipv4Addr)
	if err != nil {
		return nil, err
	}

	for _, event := range sub.EventList {

		switch event.Type_ {
		case "USER_DATA_USAGE_MEASURES":
			switch event.GranularityOfMeasurement {
			case "PER_SESSION":
				if report.ctrs["uplink"] != nil {
					return nil, fmt.Errorf("duplicate session level volume measurement event")
				}

				report.ctrs["uplink"] = make([]uint32, 0)
				report.ctrs["downlink"] = make([]uint32, 0)
				for _, pdr := range all.pdrs {
					if pdr.srcIface == access {
						report.ctrs["uplink"] = append(report.ctrs["uplink"], pdr.ctrID)
					} else if pdr.srcIface == core {
						report.ctrs["downlink"] = append(report.ctrs["downlink"], pdr.ctrID)
					}
				}
				break
			case "PER_APPLICATION":

			}
			break
		case "USER_DATA_USAGE_TRENDS":
			monitor := &trendMonitor{}
			u.TrendMonitors[ip2int(report.ipAddr)] = monitor
			report.trendMonitor = monitor
			break
		}
	}

	return &report, nil
}

type Routes []Route

func Logger(inner http.Handler, name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inner.ServeHTTP(w, r)
	})
}

func NewNUPF(conf *Conf) *Nupf {
	nupf := &Nupf{
		Subscriptions: make(map[string][]byte),
		BaseUri:       "localhost:8080/ee-Subscriptions/",
		NextID:        0,
		Mutex:         new(sync.Mutex),
		Client:        &http.Client{},
		InstanceID:    conf.InstanceID,

		TrendMutex: sync.Mutex{},

		ReportInfos:    make(map[uint32][]*reportInfo),
		ReportMutexes:  make(map[uint32]*sync.Mutex),
		TrendMonitors:  make(map[uint32]*trendMonitor),
		IntReportInfos: make(map[uint32]*intReportInfo),

		ReportInfosById:    make(map[string]*reportInfo),
		IntReportInfosById: make(map[string]*intReportInfo),
		ReportMutexesById:  make(map[string]*sync.Mutex),
	}

	go nupf.MonitorTrends(100)

	return nupf
}

func setupNupf(router *mux.Router, u *Nupf) {
	var routes = Routes{
		Route{
			"Index",
			"GET",
			"/",
			Index,
		},

		Route{
			"DeleteSubscription",
			"DELETE",
			"/ee-Subscriptions/{subscriptionId}",
			u.DeleteSubscriptionHandle,
		},

		Route{
			"ModifySubscription",
			"PATCH",
			"/ee-Subscriptions/{subscriptionId}",
			u.ModifySubscriptionHandle,
		},

		Route{
			"CreateSubscription",
			"POST",
			"/ee-Subscriptions",
			u.CreateSubscriptionHandle,
		},

		Route{
			"INTReport",
			"POST",
			"/int",
			u.INTReportHandle,
		},
	}

	for _, route := range routes {
		var handler http.Handler
		handler = route.HandlerFunc
		handler = Logger(handler, route.Name)

		router.
			Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(handler)
	}
}
