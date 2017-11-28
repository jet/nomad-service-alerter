package main

import (
	"io"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	consul "github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/api"

	"fmt"
	"net/http"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/jet/nomad-service-alerter/logger"
	"github.com/jet/nomad-service-alerter/notifications"
)

const (
	nomadMapRefreshInterval = 30
	serviceAlertInterval    = 60
)

func main() {

	host := os.Getenv("nomad_server")
	env := os.Getenv("env")
	region := os.Getenv("region")
	alertSwitch := os.Getenv("alert_switch")
	consulHost := os.Getenv("consul_server")
	datacenter := os.Getenv("consul_datacenter")
	meta := make(map[string]map[string]string)
	var lock = sync.RWMutex{}
	logger.Init(ioutil.Discard, os.Stdout, os.Stdout, os.Stderr)
	go refreshMap(host, &lock, &meta)
	go serviceAlerts(host, env, region, &meta, alertSwitch)                  // This go routine generates alerts for orphaned/queued allocs and restarting services
	go consulAlerts(consulHost, host, env, region, datacenter, &meta, &lock) // This go routine generates alerts for consul service health checkpoints
	http.HandleFunc("/health", health)                                       // health check
	http.ListenAndServe(":8000", nil)
}

func health(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "OK")
}

func refreshMap(host string, lock *sync.RWMutex, meta *map[string]map[string]string) {
	client, cerr := api.NewClient(&api.Config{Address: host, TLSConfig: &api.TLSConfig{}})
	if cerr != nil {
		logger.Error.Printf("Unable to create client(%v): %v", host, cerr)
	}
	optsNomad := &api.QueryOptions{AllowStale: true}
	for {
		jobList, _, err := client.Jobs().List(optsNomad)
		if err != nil {
			logger.Error.Printf("Cannot get job List from Nomad : %v \n", err.Error())
		}
		pm := *meta
		for _, job := range jobList {
			value, _, err := client.Jobs().Info(job.ID, optsNomad)
			if value.IsPeriodic() == true || *value.Type == "system" || *value.Type == "batch" {
				continue
			}
			if err != nil {
				logger.Error.Printf("Cannot get job info from Nomad : %v \n", err.Error())
			}
			if len(value.TaskGroups) > 0 {
				if len(value.TaskGroups[0].Tasks) > 0 {
					if len(value.TaskGroups[0].Tasks[0].Services) > 0 {
						pm[value.TaskGroups[0].Tasks[0].Services[0].Name] = value.Meta
					}
				}
			}
		}
		lock.Lock()
		meta = &pm
		lock.Unlock()
		time.Sleep(time.Second * time.Duration(nomadMapRefreshInterval))
	}
}

func consulAlerts(consulHost string, host string, env string, region string, datacenter string, meta *map[string]map[string]string, lock *sync.RWMutex) {
	config := consul.DefaultConfig()
	config.Address = consulHost
	config.Datacenter = datacenter
	config.Token = ""
	consulClient, _ := consul.NewClient(config)
	alerts := make(map[string]time.Time) // This map will hold the details of the servcies in critical state. Key--> service name  value-->first time it was reported as critical
	alertTriggered := make(map[string]string)
	var lastIndexConsul uint64
	for {
		//go through the alert map and see which jobs have reached threshold and alert based on them
		optsConsul := &consul.QueryOptions{AllowStale: true, WaitIndex: lastIndexConsul, WaitTime: (60 * time.Second)}
		healthChecks, qmConsul, err := consulClient.Health().State("critical", optsConsul)
		if err != nil {
			logger.Error.Println("Error creating Consul client : ", err.Error())
		}
		lastIndexConsul = qmConsul.LastIndex
		criticalServices := make(map[string]bool) // This map helps us remove the services which have moved from critical to passing state
		for _, check := range healthChecks {
			criticalServices[check.ServiceName] = true
			if _, ok := alerts[check.ServiceName]; ok {
				continue
			} else {
				alerts[check.ServiceName] = time.Now()
			}
		}
		// Iterate through each member of the alerts map to check which ones need to be alerted
		lock.RLock()     // Acquiring the read lock
		metaNew := *meta // This is the local version of map we will be using in this loop
		lock.RUnlock()   // Releasing the read lock
		for k, v := range alerts {
			logger.Info.Printf("[Consul-Check %v-%v] : Job %v is in CRITICAL state\n", os.Getenv("env"), os.Getenv("region"), k)
			if _, ok1 := metaNew[k]; !ok1 {
				logger.Info.Printf("Service not registered on Nomad. Removed from alert list : %v \n", k)
				delete(criticalServices, k)
			}
			if _, ok := criticalServices[k]; ok { //This is to check if the service is still in critical state
				t1 := time.Now()
				metaKey := metaNew[k]
				consulCheck := ""
				consulThreshold := ""
				integrationKey := ""
				if _, ok := metaKey["consul_service_healthcheck_enabled"]; ok {
					consulCheck = metaKey["consul_service_healthcheck_enabled"]
				}
				if _, ok := metaKey["consul_service_healthcheck_threshold"]; ok {
					consulThreshold = metaKey["consul_service_healthcheck_threshold"]
				}
				if _, ok := metaKey["pd_service_key"]; ok {
					integrationKey = metaKey["pd_service_key"]
				}
				if consulCheck == "true" {
					threshold, _ := time.ParseDuration(consulThreshold)
					if t1.Sub(v).Seconds() >= threshold.Seconds() {
						opt := &consul.QueryOptions{AllowStale: true}
						hc, _, _ := consulClient.Health().Checks(k, opt)
						var criticalList []string // This will store list of Unhealthy allocations corresponding to critical service
						for _, service := range hc {
							if service.Status == "passing" {
								continue
							}
							s1 := service.ServiceID
							s1 = s1[16:24] // This will catch the allocation ID which is critical
							criticalList = append(criticalList, s1)
						}
						// criticalList := range hc
						message := fmt.Sprintf("[Consul Healthcheck %v %v] Job : %v is in CRITICAL state. Allocations in Critical state : %v", os.Getenv("env"), os.Getenv("region"), k, criticalList)
						fmt.Printf("%v \n", message)
						err := notifications.PDAlert("trigger", k, integrationKey, message, "consul")
						if err != nil {
							logger.Error.Println("Error in PD : ", err.Error())
						}
						alertTriggered[k] = "triggered"
					}
				}
			} else {
				if alertTriggered[k] == "triggered" { // This means alert has been triggered. Resolve the alert
					metaKey := metaNew[k]
					integrationKey := ""
					if _, ok := metaKey["pd_service_key"]; ok {
						integrationKey = metaKey["pd_service_key"]
					}
					err := notifications.PDAlert("resolve", k, integrationKey, "resolved", "consul")
					if err != nil {
						logger.Error.Println("Error in PD : ", err.Error())
					}
					logger.Info.Printf("Alert is resolved for service : %v \n", k)
				}
				delete(criticalServices, k) // Remove the services which have moved away from CRITICAL state
				delete(alerts, k)
				delete(alertTriggered, k)
			}
		}
	}
}

func serviceAlerts(host string, env string, region string, meta *map[string]map[string]string, alertSwitch string) {

	client, cerr := api.NewClient(&api.Config{Address: host, TLSConfig: &api.TLSConfig{}})
	if cerr != nil {
		logger.Error.Printf("Unable to create client(%v): %v", host, cerr)
	}
	teamAlert := make(map[string]int)
	count := 0
	for {
		count++
		nodes := client.Nodes()
		jobs := client.Jobs()
		opts := &api.QueryOptions{AllowStale: true}
		resp, _, err := nodes.List(opts)
		serviceAlert := make(map[string][]string)
		if err != nil {
			logger.Error.Printf("Failed to grab node list: %v \n", err.Error())
		}
		for _, n := range resp {

			alerts(n, nodes, opts, serviceAlert, teamAlert)
		}
		for k, v := range serviceAlert {
			job, _, err := jobs.Info(k, opts)
			if err != nil {
				logger.Error.Println("error grabbing inofrmation of job : ", k)
				continue
			}
			if *job.Type == "system" {
				continue
			}
			integrationKey := ""
			if _, ok := job.Meta["pd_service_key"]; ok {
				integrationKey = job.Meta["pd_service_key"]
			}
			allocCount := 0
			taskGroupLen := len(job.TaskGroups)
			if taskGroupLen > 0 {
				for it := 0; it < taskGroupLen; it++ {
					allocCount = allocCount + *job.TaskGroups[it].Count
				}
				if allocCount != len(v) {
					if allocCount < len(v) {
						orphanCount := len(v) - allocCount
						logger.Info.Printf("[%v] Job=\"%v\" Error=\"orphaned allocations\"  Orphaned Allocations Count=\"%v\"\n", time.Now(), k, orphanCount)
						message := " Job : " + k + " has " + strconv.Itoa(orphanCount) + " orphaned allocations "
						if _, ok := job.Meta["orphaned_instances_alert_enabled"]; ok {
							err := notifications.PDAlert("trigger", k, integrationKey, message, "service")
							if err != nil {
								logger.Error.Println("Error in PD : ", err.Error())
							}
						}
					} else {
						queuedCount := allocCount - len(v)
						logger.Info.Printf("[%v] Job=\"%v\" Error=\"queued instances\"  Queued Instances Count=\"%v\" \n", time.Now(), k, queuedCount)
						message := " Job : " + k + " has " + strconv.Itoa(queuedCount) + " queued instances "
						if _, ok := job.Meta["queued_instances_alert_enabled"]; ok {
							err := notifications.PDAlert("trigger", k, integrationKey, message, "service")
							if err != nil {
								logger.Error.Println("Error in PD : ", err.Error())
							}
						}
					}
				}
			}
		}

		jobalertmap := make(map[string][]string)
		for k1, v1 := range teamAlert {
			result := strings.Split(k1, ",")
			logger.Info.Printf("[%v] Job=\"%v\" Error=\"pending allocations\"  AllocationId=\"%v\" \n", time.Now(), result[1], result[0])
			if v1 == 3 {
				jobalertmap[result[1]] = append(jobalertmap[result[1]], " "+result[0])
			}
		}
		for k2, v2 := range jobalertmap {
			logger.Info.Printf("[%v] Job=\"%v\" Error=\"Service in Restart Loop\"  Allocations=\"%v\" \n", time.Now(), k2, v2)
			restartmessage := "[Restart-Loop " + env + " " + region + "] Job = " + k2 + " has following allocations in restart loop : " + strings.Join(v2, " ")
			job, _, err := jobs.Info(k2, opts)
			if err != nil {
				continue
			}
			if job.Meta["restart_loop_alerting_enabled"] == "true" {
				pdKey := job.Meta["pd_service_key"]
				event1 := pagerduty.Event{
					Type:        "trigger",
					ServiceKey:  pdKey,
					Description: restartmessage,
					IncidentKey: *job.ID,
				}
				resp1, err := pagerduty.CreateEvent(event1)
				if err != nil {
					log.Println(resp1)
					log.Fatalln("ERROR in PD:", err)
				}
			}
		}
		if count == 3 {
			count = 0
			teamAlert = make(map[string]int)
		}
		time.Sleep(time.Second * time.Duration(serviceAlertInterval))
	}
}

func alerts(n *api.NodeListStub, nodes *api.Nodes, opts *api.QueryOptions, thisGroup map[string][]string, thisAlert map[string]int) {
	nodeAlloc, _, err := nodes.Allocations(n.ID, opts)
	if err != nil {
		logger.Error.Printf("error grabbing allocation info : %v", err.Error())
	}

	for _, i := range nodeAlloc {
		status := i.ClientStatus
		t := float64(300)
		if status == "running" || status == "pending" && *i.Job.Type != "system" && !strings.Contains(i.JobID, "periodic") {
			z := epochToHumanReadable(int64(i.CreateTime / 1000000000))
			if time.Since(z).Seconds() > t {

				//if(ok == false)
				if len(thisGroup[i.JobID]) > 0 {
					thisGroup[i.JobID] = append(thisGroup[i.JobID], "| "+i.ID)
				} else {
					thisGroup[i.JobID] = append(thisGroup[i.JobID], " "+i.ID)
				}
			}
		}
		if status == "pending" && !strings.Contains(i.JobID, "periodic") {
			z := epochToHumanReadable(int64(i.CreateTime / 1000000000))
			if time.Since(z).Seconds() > t {

				thisAlert[i.ID+","+i.JobID]++
			}
		}
	}
}

func epochToHumanReadable(epoch int64) time.Time {
	return time.Unix(epoch, 0)
}
