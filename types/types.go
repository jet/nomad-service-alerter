package types

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
)

//This space is for adding new types to the code. Right now we don't have any types but might need in future

// Alerts ...
func Alerts(n *api.NodeListStub, nodes *api.Nodes, opts *api.QueryOptions, thisGroup map[string][]string, thisAlert map[string]int) {
	//node, _, _ := nodes.Info(n.ID, opts)
	nodeAlloc, _, err := nodes.Allocations(n.ID, opts)
	if err != nil {
		fmt.Printf("error grabbing allocation info : %v", err.Error())
	}

	for _, i := range nodeAlloc {
		status := i.ClientStatus
		t := float64(300)
		if status == "running" || status == "pending" && *i.Job.Type != "system" && !strings.Contains(i.JobID, "periodic") {
			//fmt.Printf("time : %v \n", i.CreateTime-(now.Unix()))
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
			//fmt.Printf("%v Is Periodic ? : %v \n", i.JobID, i.Job.IsPeriodic())
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
