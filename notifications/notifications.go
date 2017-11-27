package notifications

import (
	"log"

	pagerduty "github.com/PagerDuty/go-pagerduty"
)

// PDAlert ...
func PDAlert(action string, serviceName string, integrationKey string, message string, tag string) error {
	event := pagerduty.Event{
		Type:        action,
		ServiceKey:  integrationKey,
		Description: message,
		IncidentKey: tag + serviceName,
	}
	resp, err := pagerduty.CreateEvent(event)
	if err != nil {
		log.Println(resp)
		return err
	}
	return nil
}
