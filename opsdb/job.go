package opsdb

import (
	"fmt"
	"os"
	"database/sql"
	"log"
	"time"
	"sync"
	. "github.com/ghowland/yudien/yudienutil"
	. "github.com/ghowland/yudien/yudiendata"
	. "github.com/ghowland/yudien/yudien"
)


type Job struct {
	Id int
	Data map[string]interface{}
	UdnSchema map[string]interface{}
	UdnData map[string]interface{}
	Db *sql.DB
}

//type JobResult struct {
//	Job Job
//	Result UdnResult
//	WasSuccess bool
//}

type WorkerInfo struct {
	IsBusy bool
	Job Job
}

var WorkerInfoPool = make([]WorkerInfo, 10)

var JobChannel = make(chan Job, 10)
//var JobResultChannel = make(chan JobResult, 10)


func AssignJobWorker(job map[string]interface{}, udn_schema map[string]interface{}, db *sql.DB) {
	fmt.Printf("\nAssign Job Worker: %v\n", job)

	job["running_host_claimed"] = true
	DatamanSet("job", job)

	job_data := Job{}
	job_data.Id = int(job["_id"].(int64))
	job_data.Data = job

	// Data pool for UDN
	job_data.UdnSchema = udn_schema
	job_data.UdnData = make(map[string]interface{})
	job_data.Db = db


	// Send the job to the workers
	JobChannel <- job_data
}

func TestJobWorker(job map[string]interface{}) {
	fmt.Printf("\nTest Job Worker: %v\n", job)

}

func JobScheduleStartingActions(job Job) {
	filter := make(map[string]interface{})
	filter["job_spec_group_id"] = []interface{} {"=", job.Data["job_spec_group_id"]}		//TODO(g): Dataman doesnt support searching for NULL yet
	options := make(map[string]interface{})
	options["sort"] = []string{"priority"}
	group_items := DatamanFilter("job_spec_group_item", filter, options)

	found_current_job := false

	// If we havent started, or we have completed our current item
	if job.Data["current_job_spec_group_item_id"] == nil || job.Data["finished_job_spec_group_item_id"] != nil {
		for _, group_item := range group_items {
			fmt.Printf("Scheduling Group Items: %v\n", group_item)

			if job.Data["current_job_spec_group_item_id"] == nil {
				// Select the first item
				job.Data["current_job_spec_group_item_id"] = group_item["_id"]
				break
			} else if job.Data["finished_job_spec_group_item_id"] == group_item["_id"] {
				// Select the next item
				found_current_job = true
			} else if found_current_job == true {
				// Selects the next one
				job.Data["current_job_spec_group_item_id"] = group_item["_id"]
				break
			}
		}

	}
}


func JobScheduleFinishingActions(job Job, success bool) {
	// Set our current item as finished
	job.Data["finished_job_spec_group_item_id"] = job.Data["current_job_spec_group_item_id"]

	// Get all our items
	filter := make(map[string]interface{})
	filter["job_spec_group_id"] = []interface{} {"=", job.Data["job_spec_group_id"]}		//TODO(g): Dataman doesnt support searching for NULL yet
	options := make(map[string]interface{})
	options["sort"] = []string{"priority"}
	group_items := DatamanFilter("job_spec_group_item", filter, options)

	// Find the current index
	current_index := -1
	is_next_item := false

	for index, group_item := range group_items {
		if job.Data["current_job_spec_group_item_id"] == group_item["_id"] {
			current_index = index
		}
	}

	// Bound reality
	if current_index == -1 {
		panic("We didnt find the job we thought we were running.  Everything broken!  Chickens and Dogs!  (This should never happen!  Chickens and dogs!)")
	} else if current_index < len(group_items) - 1 {
		is_next_item = true
	}


	fmt.Printf("Finished:  Is Next Item: %v   Success: %v\n\n", is_next_item, success)

	// Look at the group if we failed to determine if we abort
	options = make(map[string]interface{})
	job_spec_group := DatamanGet("job_spec_group", int(job.Data["job_spec_group_id"].(int64)), options)

	if job_spec_group["on_failure_abort"] == true && success == false {
		job.Data["was_success"] = false
		job.Data["finished"] = time.Now()
	} else if is_next_item == false {
		// If we make it to the end, it's a success
		job.Data["was_success"] = true
		job.Data["finished"] = time.Now()
	}

	// If this job isnt finished, we release it to another machine (or maybe this one again)
	if job.Data["was_success"] == nil {
		// Release this job so another host can pick it up, if it has more work to do
		//TODO(g): Only do this if we dont have the requirements to meet the next job's dependency.  Also check this before locking the first job.
		job.Data["running_host_key"] = nil
		job.Data["running_host_claimed"] = false

	}

	// Update the Job record
	DatamanSet("job", job.Data)

}

func RunJobWorkerSingle(job Job) bool {
	// Update our UDN Data to the previously stored result_data_json, if we have it
	if job.Data["result_data_json"] != nil {
		job.UdnData = job.Data["result_data_json"].(map[string]interface{})
	}

	// Get the Job Spec for our current job, so we know what to run
	options := make(map[string]interface{})
	job_spec_group_item := DatamanGet("job_spec_group_item", int(job.Data["current_job_spec_group_item_id"].(int64)), options)
	job_spec := DatamanGet("job_spec", int(job_spec_group_item["job_spec_id"].(int64)), options)

	// Run the UDN
	udn_json := JsonDump(job_spec["udn_data_json"])
	result := ProcessSchemaUDNSet(job.Db, job.UdnSchema, udn_json, job.UdnData)

	fmt.Printf("Run: Job Worker: Single: RESULT: %v\n\n", result)

	// Save the UDN Data map into our result_data_json, which will be passed on
	job.Data["result_data_json"] = job.UdnData


	// Run the Test UDN, to determine success/fail
	//test_result := ProcessSchemaUDNSet(job.Db, job.UdnSchema, JsonDump(job.Data["test_udn_data_json"]), job.UdnData)

	// Debug Info
	if job.Data["debug_data_json"] == nil {
		job.Data["debug_data_json"] = make([]interface{}, 0)
	}
	new_debug_info := make(map[string]interface{})
	new_debug_info["job_spec_group_item_id"] = job.Data["current_job_spec_group_item_id"]
	new_debug_info["job_spec_id"] = job_spec["_id"]
	new_debug_info["hostkey"] = job.Data["running_host_key"]
	new_debug_info["result"] = result


	//fmt.Printf("Run: Job Worker: Single: TEST RESULT: %v\n\n", test_result)


	// Determine if this is the last job?  Wrap up?


	success := true
	new_debug_info["success"] = success

	//new_debug_info["test_result"] = test_result
	job.Data["debug_data_json"] = append(job.Data["debug_data_json"].([]interface{}), new_debug_info)

	return success
}

func Worker(wg *sync.WaitGroup, worker_info *WorkerInfo) {
	// Not yet doing anything
	worker_info.IsBusy = false

	for job := range JobChannel {
		// Doing things
		worker_info.IsBusy = true

		fmt.Printf("Worker: Received a job!\n\n")

		// Perform Scheduler actions on this job
		JobScheduleStartingActions(job)

		// Run the UDN, test it, update the DB
		success := RunJobWorkerSingle(job)

		// Perform Scheduler actions on this job
		JobScheduleFinishingActions(job, success)

		// Run the job

		//TODO(g): Clean up, once we know we dont need to deal with a results channel
		//result := JobResult{}
		//result.Job = job
		//result.Result = UdnResult{}
		//result.Result.Result = 5
		//JobResultChannel <- result

		// Done doing things
		worker_info.IsBusy = false
	}

	// Get the current result_data_json, which is our input, unless there is none, then we use input_data_json
	//TODO(g)...

	// Start this job's UDN

	// Save the udn_data to the result_data_json

	// Mark this job as finished
}

func CreateWorkers() {
	wg := sync.WaitGroup{}

	for i := 0 ; i < 10 ; i++ {
		wg.Add(1)
		go Worker(&wg, &WorkerInfoPool[i])
	}

	//NOTE(g): Not useful because we arent handling the results, and we arent closing the JobChannel, but leaving for reference, clean up later
	wg.Wait()

	//close(JobResultChannel)
}

func CountFreeWorkers() int {
	value := 0

	for i := 0 ; i < 10 ; i++ {
		if WorkerInfoPool[i].IsBusy == false {
			value++
		}
	}

	return value
}

func NextJobSpecDependenciesMatchThisHost(job map[string]interface{}) bool {
	//TODO(g): Look at the next job_spec_group_item, determine it's deps, and whether this host do them
	return true
}

func RunJobWorkers() {
	fmt.Printf("Running Job Workers\n")

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknownhostname"
	}
	//hostkey := fmt.Sprintf("%s__%s", hostname, ksuid.New().String())
	hostkey := hostname		//TODO(g): For single machine, multiple run, testing

	// Create workers
	go CreateWorkers()

	// DB
	db, err := sql.Open("postgres", PgConnect)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Get UDN schema per request
	udn_schema := PrepareSchemaUDN(db)

	// Keep track of what workers are currently locked, because we are waiting on a job to pick up, and dont want to use them for something else after we have gotten the lock
	locked_workers := 0
	changed_locked_workers := false

	for true {
		filter := make(map[string]interface{})
		filter["was_success"] = []interface{} {"=", nil}
		options := make(map[string]interface{})
		job_array := DatamanFilter("job", filter, options)

		fmt.Printf("Looping Job Worker: %s: %d: %v\n\n", hostkey, len(job_array), job_array)

		changed_locked_workers = false

		for _, job := range job_array {
			// If we find the running_host_key empty, set it ourselves
			if job["running_host_key"] == nil && NextJobSpecDependenciesMatchThisHost(job) {
				// Check if we have a spare worker, and lock it
				free_workers := CountFreeWorkers()

				// If we have free workers, taking into account any locks we are already waiting on...
				if free_workers - locked_workers > 0 {
					// We can take this job.  Increment locked workers.
					locked_workers++
					changed_locked_workers = true

					// Assign to ourself
					job["running_host_key"] = hostkey
					job["updated"] = time.Now()

					fmt.Printf("Claiming job: %d\n\n", job["_id"])

					// Save the job
					//TODO(g): Set the OSI, after I figure out which one I am
					DatamanSet("job", job)
				}
			} else if job["running_host_key"] == hostkey && job["running_host_claimed"] == false {
				// This is a new job, assign it to ourselves
				locked_workers--

				// Loop through the jobs, if available, see if there is an available worker, and hand to the worker
				AssignJobWorker(job, udn_schema, db)

				//} else if job["running_host_key"] == hostkey {
				//	// Make sure this job isnt fucked up.  Maybe this isnt necessary...  Checking for the host wont do it, because there could be a different process on this host...
				//
				//	// Is this worker still working on this job?  If not, we have to note the failed run, since it was lost...
				//	TestJobWorker(job)
			} else {
				fmt.Printf("\nNot finding the job match: %s :: %v\n\n", hostkey, job)
			}
		}

		// If we didnt just set this, then it should always return to 0, or we already picked up the job
		if changed_locked_workers == false {
			locked_workers = 0
		}

		// Wait...
		//TODO(g): WEILUMATH: This could double run if everything was done to it to force it to lock.
		time.Sleep(time.Second)
	}
}
