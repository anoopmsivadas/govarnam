package govarnam

import (
	"io/ioutil"
	"log"
	"os"
	"path"
	"reflect"
	"runtime/debug"
	"sync"
	"testing"
)

var varnamInstances = map[string]*Varnam{}
var mutex = sync.RWMutex{}
var testTempDir string

// AssertEqual checks if values are equal
// Thanks https://gist.github.com/samalba/6059502#gistcomment-2710184
func assertEqual(t *testing.T, a interface{}, b interface{}) {
	if a == b {
		return
	}
	debug.PrintStack()
	t.Errorf("Received %v (type %v), expected %v (type %v)", a, reflect.TypeOf(a), b, reflect.TypeOf(b))
}

func checkError(err error) {
	if err != nil {
		log.Panic(err)
	}
}

func makeFile(name string, contents string) string {
	filePath := path.Join(testTempDir, name)

	file, err := os.Create(filePath)
	if err != nil {
		log.Println(err)
		return ""
	}
	defer file.Close()

	file.WriteString(contents)

	return filePath
}

func setUp(schemeID string) {
	os.Setenv("VARNAM_LEARNINGS_DIR", testTempDir)

	varnam, err := InitFromID(schemeID)
	checkError(err)

	mutex.Lock()
	varnamInstances[schemeID] = varnam
	mutex.Unlock()
}

func getVarnamInstance(schemeID string) *Varnam {
	mutex.Lock()
	instance, ok := varnamInstances[schemeID]
	mutex.Unlock()

	if ok {
		return instance
	}
	return nil
}

func tearDownVarnam(schemeID string) {
	getVarnamInstance(schemeID).Close()
}

func tearDown() {
	os.RemoveAll(testTempDir)
}

func TestEnv(t *testing.T) {
	// Making a dummy VST file
	makeFile("ml.vst", "dummy")

	prevEnvValue := os.Getenv("VARNAM_VST_DIR")

	os.Setenv("VARNAM_VST_DIR", testTempDir)
	_, err := InitFromID("ml")
	assertEqual(t, err != nil, true)

	os.Setenv("VARNAM_VST_DIR", prevEnvValue)
	os.Setenv("VARNAM_LEARNINGS_DIR", testTempDir)

	_, err = InitFromID("ml")
	checkError(err)

	assertEqual(t, fileExists(path.Join(testTempDir, "ml.vst.learnings")), true)
}

func TestMain(m *testing.M) {
	schemeDetails, err := GetAllSchemeDetails()

	if err != nil {
		log.Fatal(err)
	}

	testTempDir, err = ioutil.TempDir("", "govarnam_test")
	checkError(err)

	for _, schemeDetail := range schemeDetails {
		setUp(schemeDetail.Identifier)
	}

	m.Run()

	for _, schemeDetail := range schemeDetails {
		tearDownVarnam(schemeDetail.Identifier)
	}

	tearDown()
}
