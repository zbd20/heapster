package alertmanager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/golang/glog"
	v1 "k8s.io/api/core/v1"
	"k8s.io/heapster/events/core"
)

const (
	ALERTMANAGER_SINK     = "alertmanager"
	WARNING           int = 2
	NORMAL            int = 1
	CONTENT_TYPE_JSON     = "application/json"

	// AlertNameLabel is the name of the label containing the an alert's name.
	AlertNameLabel     = "alertname"
	AlertClusterLabel  = "cluster"
	AlertGroupLabel    = "group"
	AlertLevelLabel    = "level"
	AlertInstanceLabel = "instance"
	AlertReasonLabel   = "reason"
)

var NotVaildAlertName error = fmt.Errorf("not valid alert name")

type AlertmanagerSink struct {
	Endpoint string
	Level    int
	Cluster  string
}

// Alert is a generic representation of an alert in the Prometheus eco-system.
type Alert struct {
	// Label value pairs for purpose of aggregation, matching, and disposition
	// dispatching. This must minimally include an "alertname" label.
	Labels map[string]string `json:"labels"`

	// Extra key/value information which does not define alert identity.
	Annotations map[string]string `json:"annotations"`
}

func (a *AlertmanagerSink) Name() string {
	return ALERTMANAGER_SINK
}

func (a *AlertmanagerSink) Stop() {
	//do nothing
}

func (a *AlertmanagerSink) ExportEvents(batch *core.EventBatch) {

	var alerts []*Alert
	for _, event := range batch.Events {
		if a.isEventLevelDangerous(event.Type) {
			alert, err := createAlertFromEvent(a.Cluster, event)
			if err != nil {
				glog.Warningf("failed to create alert from event,because of %v", event)
				continue
			}

			alerts = append(alerts, alert)
		}
	}

	if len(alerts) > 0 {
		a.Send(alerts)
	}

}

func NewAlertmanagerSink(uri *url.URL) (*AlertmanagerSink, error) {
	d := &AlertmanagerSink{
		Level: WARNING,
	}
	if len(uri.Host) > 0 {
		d.Endpoint = uri.Host + uri.Path
	}
	opts := uri.Query()

	if len(opts["cluster"]) >= 1 {
		d.Cluster = opts["cluster"][0]
	} else {
		return nil, fmt.Errorf("you must provide cluster name")
	}

	if len(opts["level"]) >= 1 {
		d.Level = getLevel(opts["level"][0])
	}

	return d, nil
}

func (a *AlertmanagerSink) isEventLevelDangerous(level string) bool {
	score := getLevel(level)
	if score >= a.Level {
		return true
	}
	return false
}

func getLevel(level string) int {
	score := 0
	switch level {
	case v1.EventTypeWarning:
		score += 2
	case v1.EventTypeNormal:
		score += 1
	default:
		//score will remain 0
	}
	return score
}

func (a *AlertmanagerSink) Send(alerts []*Alert) {

	alert_bytes, err := json.Marshal(alerts)
	if err != nil {
		glog.Warningf("failed to marshal alert %v", alerts)
		return
	}

	b := bytes.NewBuffer(alert_bytes)

	_, err = http.Post(fmt.Sprintf("http://%s", a.Endpoint), CONTENT_TYPE_JSON, b)
	if err != nil {
		glog.Errorf("failed to send msg to alertmanager,because of %s", err.Error())
		return
	}

	glog.Infof("alert send success: %v", alerts)
}

func createAlertFromEvent(cluster string, event *v1.Event) (*Alert, error) {
	labels := make(map[string]string)
	if event.Message != "" {
		labels[AlertNameLabel] = event.Message
	} else {
		return nil, NotVaildAlertName
	}

	if event.Namespace != "" {
		labels[AlertGroupLabel] = strings.ToUpper(event.Namespace)
	}

	if event.Type != "" {
		labels[AlertLevelLabel] = event.Type
	}
	if event.Name != "" {
		labels[AlertInstanceLabel] = event.Name
	}

	if event.Reason != "" {
		labels[AlertReasonLabel] = event.Reason
	}

	labels[AlertClusterLabel] = cluster

	alert := &Alert{
		Labels: labels,
	}

	return alert, nil
}
