package alertmanager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

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

	// The known time range for this alert. Both ends are optional.
	StartsAt     time.Time `json:"startsAt,omitempty"`
	EndsAt       time.Time `json:"endsAt,omitempty"`
	GeneratorURL string    `json:"generatorURL"`
}

func (a *AlertmanagerSink) Name() string {
	return ALERTMANAGER_SINK
}

func (a *AlertmanagerSink) Stop() {
	//do nothing
}

func (a *AlertmanagerSink) ExportEvents(batch *core.EventBatch) {
	for _, event := range batch.Events {
		if a.isEventLevelDangerous(event.Type) {
			a.Send(event)
		}
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

func (a *AlertmanagerSink) Send(event *v1.Event) {
	alert, err := createAlertFromEvent(a.Cluster, event)
	if err != nil {
		glog.Warningf("failed to create alert from event,because of %v", event)
		return
	}

	alert_bytes, err := json.Marshal(alert)
	if err != nil {
		glog.Warningf("failed to marshal alert %v", alert)
		return
	}

	b := bytes.NewBuffer(alert_bytes)

	_, err = http.Post(fmt.Sprintf("http://%s", a.Endpoint), CONTENT_TYPE_JSON, b)
	if err != nil {
		glog.Errorf("failed to send msg to alertmanager,because of %s", err.Error())
		return
	}
}

func createAlertFromEvent(cluster string, event *v1.Event) (*Alert, error) {
	labels := make(map[string]string)
	if event.Message != "" {
		labels[AlertNameLabel] = event.Message
	} else {
		return nil, NotVaildAlertName
	}

	if event.Namespace != "" {
		labels[AlertGroupLabel] = event.Namespace
	}

	if event.Type != "" {
		labels[AlertLevelLabel] = event.Type
	}
	if event.Name != "" {
		labels[AlertInstanceLabel] = event.Name
	}

	labels[AlertClusterLabel] = cluster

	alert := &Alert{
		Labels: labels,

		StartsAt: event.FirstTimestamp.Time,
	}

	return alert, nil
}
