NOTICE: SUPPORT FOR THIS PROJECT ENDED ON 18 November 2020

This projected was owned and maintained by Jet.com (Walmart). This project has reached its end of life and Walmart no longer supports this project.

We will no longer be monitoring the issues for this project or reviewing pull requests. You are free to continue using this project under the license terms or forks of this project at your own risk. This project is no longer subject to Jet.com/Walmart's bug bounty program or other security monitoring.


## Actions you can take

We recommend you take the following action:

  * Review any configuration files used for build automation and make appropriate updates to remove or replace this project
  * Notify other members of your team and/or organization of this change
  * Notify your security team to help you evaluate alternative options

## Forking and transition of ownership

For [security reasons](https://www.theregister.co.uk/2018/11/26/npm_repo_bitcoin_stealer/), Walmart does not transfer the ownership of our primary repos on Github or other platforms to other individuals/organizations. Further, we do not transfer ownership of packages for public package management systems.

If you would like to fork this package and continue development, you should choose a new name for the project and create your own packages, build automation, etc.

Please review the licensing terms of this project, which continue to be in effect even after decommission.

ORIGINAL README BELOW

----------------------

# Nomad Service Alerter

Nomad Service Alerter is a tool written in Go, whose primary goal is to provide alerting for your services running on Nomad (https://www.nomadproject.io/). It offers configurable opt-in alerting options which you can specify in your Nomad Job manifest (json file) as Environment Variables. The Nomad Service Alerter mainly covers Consul Health-Check Alerts and Service Restart-Loops Alerts.

## Alerts

Nomad Service Alerter supports the following Alerts :

### Consul Health-Check Alerts

This alert will monitor your service and alert on allocations and versions that are failing their defined consul health-checks. You will be able to set the duration threshold for which the service must remain unhealthy before alerting. The alert will include the details of all the allocations of the service which is failing the consul health check.

### Service Restart-Loops Alerts

This alert will monitor jobs (and all of its allocations) and alert on the services which go into an un-ending restart loop. This indicates that there is an error in the service which is not allowing it to enter a successful Running state (the allocations are created but are constantly in pending state). This is a more accurate way to alert of Nomad jobs vs. monitoring Dead state (which may be a valid state if you set count to 0).

### Queued Instances Alerts

You can configure Nomad Service Alerter to opt in into Queued Instances Alerts which will trigger an alert when the service has un-allocated instances for at least 3 minutes.

### Orphaned Instances Alerts

You can configure Nomad Service Alerter to opt in into Orphaned Instances Alerts which will trigger an alert when the service has more number of allocations running than what it has asked for (In this case there is one or multiple rogue allocations running on some machine which do not have any parent nomad process, hence the name). Similar to Queued instances alert, this alert will be triggered when the service remains in described state for at least 3 minutes.

## Build and Test

To run the tool on your local machine, you will have to :
* Install and set up your Go environment. (https://golang.org/doc/install)
* Install glide (https://github.com/Masterminds/glide)
* Clone the repo (git clone https://github.com/jet/nomad-service-alerter)
* cd into the code repo (```cd nomad-service-alerter```)
* Run ```glide init```
* Run ```glide install```
* Make sure following environment variables are set with appropriate values.
```

"nomad_server" --> your nomad server address
"env" --> the environment in which the tool would be running
"region" --> the region in which your tool would be running
"consul_server" --> your consul server address
"consul_datacenter" --> datacenter of your consul server

```
You can use the script ```loadenv.sh``` after adding appropriate values to load all the above variables.
* Run ```go build```
* Execute the binary. (Or you can skip the ```go build``` step and run ```go run main.go``` instead)


### Configuring a nomad service to be alerted on by Nomad Service Alerter upon being unhealthy

You can configure your service by adding following key-value pairs to the **Meta** section of your Nomad Job.
* consul_service_healthcheck_enabled --> true/false (to enable/disable consul healthcheck alerts)
* consul_service_healthcheck_threshold --> Time duration for which service can remain in unhealthy state before getting alerted (eg. 2m0s)
* pd_service_key --> 32 characters Pagerduty Serrvice integration key (all the alerts will be sent here)
* restart_loop_alerting_enabled --> true/false (to enable/disable restart loop alerts)
* orphaned_instances_alert_enabled --> true/false (to enable/disable orphaned allocations alert)
* queued_instances_alert_enabled --> true/false (to enable/disable queued allocations alert)

Following is an example of key-value pairs described above that your Job **Meta** section (Job level) should have :

```
consul_service_healthcheck_enabled: true
consul_service_healthcheck_threshold: 3m0s
restart_loop_alerting_enabled: true
orphaned_instances_alert_enabled: true
queued_instances_alert_enabled: true
pd_service_key: 22221234567890123456789000000000

```

## Running Nomad Service Alerter on Nomad

If you want to run Nomad Service Alerter on Nomad, you would need to have the Environment Variables (ones described in 'Build and Test' section) set with appropriate values in your job manifest (json file):

```

"nomad_server" --> your nomad server address
"env" --> the environment in which the tool would be running
"region" --> the region in which your tool would be running
"consul_server" --> your consul server address
"consul_datacenter" --> datacenter of your consul server

```
Once your Job file is ready, use the standard method of submitting the job to nomad (https://www.nomadproject.io/docs/operating-a-job/submitting-jobs.html).

## Alert Integrations

As of now, Nomad Service Alerter only supports integration with PagerDuty.

## Maintainers

* [@bhope](https://github.com/bhope) (Prathamesh Bhope)
