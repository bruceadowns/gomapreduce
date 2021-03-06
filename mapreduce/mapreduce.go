/*
gomapreduce implements a simple map-reduce algorithm
where the map and reduce operations can be specified
by the map-reduce client.
*/

package mapreduce

// MRInput defines the structure for inputs to the map and reduce functions.
type MRInput struct {
	Key    string
	Values []string
}

// MapReduce is the entry point to the map-reduce process. It takes an input to the map-reduce process,
// the mapping function, and the reduce function. "input" contains key/value pairs that represent the
// input to the map reduce process.
//
// mapFunc and ReduceFunc are expected to return their results on their respective collectChan
// channel parameters. They are also expected to signal when they have completed processing by sending a
// message on doneChl.
//
// MapReduce is a simple function that runs in the same goroutine as the caller. The rest of the map-reduce
// process runs in separate goroutines.
//
func MapReduce(input []MRInput, mapFunc func(input MRInput, collectChan chan MRInput, doneChl chan struct{}),
	reduceFunc func(input MRInput, collectChan chan MRInput, doneChl chan struct{})) (result map[string][]string) {
	resultChl := make(chan map[string][]string, 1)

	// Kick off map/reduce process
	go master(resultChl, mapFunc, reduceFunc, input)

	// Wait for result
	result = <-resultChl
	return result
}

// master implements the high level map-reduce algorithm. This mainly consists of (1) starting a goroutine for each
// of the entries in the inputs parameter to do the mapping; (2) Collecting the results of the mapping process from
// each of the mapper goroutines; (3) starting a goroutine for each of the entries in the mapping results to perform
// the reduce operation; (4) collecting the final results and sending them over the resultChl.
func master(resultChl chan map[string][]string, mapFunc func(input MRInput, collectChl chan MRInput, doneChl chan struct{}),
	reduceFunc func(input MRInput, collectChl chan MRInput, doneChl chan struct{}), inputs []MRInput) {

	// Used to collect the results from the mapping and reduce operations.
	collectChl := make(chan MRInput)
	// Used by workers to signal when they've completed
	doneChl := make(chan struct{})

	// Spawn a mapper goroutine for each input, with a mapping function and a
	// channel to collect the intermediate results.
	for _, input := range inputs {
		go mapFunc(input, collectChl, doneChl)
	}

	numResults := len(inputs)
	intermediateResultMap := collectResults(collectChl, numResults, doneChl)

	// Spawn a reduce goroutine for each mapping result, with a reduce function and a
	// channel to collect the results. First though, convert the intermediate results into
	// a slice of MRInputs suitable for input for the reduce function.
	intermediateResults := mapToKVSlice(intermediateResultMap)
	for _, intermediateResult := range intermediateResults {
		go reduceFunc(intermediateResult, collectChl, doneChl)
	}

	numResults = len(intermediateResults)
	finalResults := collectResults(collectChl, numResults, doneChl)

	resultChl <- finalResults
}

func collectResults(collectChl chan MRInput, numProcs int, doneChl chan struct{}) map[string][]string {
	results := make(map[string][]string)

	// Each map/reduce process will send a message on doneChl just prior to exiting. This function reduces
	// numProcs by 1 when signaled on the doneChl until numProcs is 0. I.e., it runs until all mappers/reducers
	// have exited.
	for i := 0; numProcs > 0; i++ {
		select {
		case result := <-collectChl:
			values := results[result.Key]
			values = append(values, result.Values...)
			results[result.Key] = values
		case <-doneChl:
			numProcs--
			continue
		}
	}
	return results
}

//mapToKVSlice transforms a map[string][]string to a slice of MRInputs.
func mapToKVSlice(kvMap map[string][]string) []MRInput {
	var kvs []MRInput
	for key, value := range kvMap {
		kv := MRInput{key, value}
		kvs = append(kvs, kv)
	}
	return kvs
}
