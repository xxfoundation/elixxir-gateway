package rateLimiting

import (
	"errors"
	jww "github.com/spf13/jwalterweatherman"
	"io/ioutil"
	"os"
	"reflect"
	"testing"
	"time"
)

var change_fileData1, change_fileData2 []byte
var ip_fileData, noNewLine_fileData []string
var change_mapData1, change_mapData2, ip_mapData, noNewLine_mapData map[string]bool

// Set up for different whitelist file contents before and after editing.
func TestMain(m *testing.M) {
	// Values for change_test.txt
	change_fileData1 = []byte("137\n152\n0\n255\n38\n84\n189\n48\n30\n83\n174\n128\n192\n115\n27\n65\n240\n78\n24\n114\n235\n215\n68\n92")
	change_fileData2 = []byte("137\n152\n0\n255\n38\n84\n189\n48\n30\n83\n174\n128\n192")
	change_mapData1 = map[string]bool{
		"137": true, "152": true, "0": true, "255": true, "38": true, "84": true,
		"189": true, "48": true, "30": true, "83": true, "174": true, "128": true,
		"192": true, "115": true, "27": true, "65": true, "240": true, "78": true,
		"24": true, "114": true, "235": true, "215": true, "68": true, "92": true,
	}
	change_mapData2 = map[string]bool{
		"137": true, "152": true, "0": true, "255": true, "38": true, "84": true,
		"189": true, "48": true, "30": true, "83": true, "174": true, "128": true,
		"192": true,
	}

	// Values for no_newline_test.txt
	ip_fileData = []string{
		"159.8.41.131", "159.8.223.74", "169.38.76.194", "169.46.49.133",
		"158.85.140.178", "50.22.103.178", "169.50.10.10", "119.81.152.130",
		"159.8.144.180", "158.176.86.162", "168.1.73.132", "169.57.1.68",
		"159.122.153.194", "159.8.77.34", "192.155.217.58", "169.45.78.148",
		"169.57.134.146", "158.85.97.148", "50.97.33.58", "169.55.101.52",
		"169.63.70.88", "169.61.65.3",
	}
	ip_mapData = map[string]bool{
		"159.8.41.131": true, "159.8.223.74": true, "169.38.76.194": true,
		"169.46.49.133": true, "158.85.140.178": true, "50.22.103.178": true,
		"169.50.10.10": true, "119.81.152.130": true, "159.8.144.180": true,
		"158.176.86.162": true, "168.1.73.132": true, "169.57.1.68": true,
		"159.122.153.194": true, "159.8.77.34": true, "192.155.217.58": true,
		"169.45.78.148": true, "169.57.134.146": true, "158.85.97.148": true,
		"50.97.33.58": true, "169.55.101.52": true, "169.63.70.88": true,
		"169.61.65.3": true,
	}

	// Values fro ip_whitelist.txt
	noNewLine_fileData = []string{"159.8.41.131 159.8.223.74 169.38.76.194 169.46.49.133 158.85.140.178 50.22.103.178 169.50.10.10 119.81.152.130 159.8.144.180 158.176.86.162 168.1.73.132 169.57.1.68 159.122.153.194 159.8.77.34 192.155.217.58 169.45.78.148 169.57.134.146 158.85.97.148 50.97.33.58 169.55.101.52 169.63.70.88 169.61.65.3"}
	noNewLine_mapData = map[string]bool{"159.8.41.131 159.8.223.74 169.38.76.194 169.46.49.133 158.85.140.178 50.22.103.178 169.50.10.10 119.81.152.130 159.8.144.180 158.176.86.162 168.1.73.132 169.57.1.68 159.122.153.194 159.8.77.34 192.155.217.58 169.45.78.148 169.57.134.146 158.85.97.148 50.97.33.58 169.55.101.52 169.63.70.88 169.61.65.3": true}

	os.Exit(m.Run())
}

// Checks if the whitelist initialises with the correct map values
func TestInitWhitelist(t *testing.T) {
	wl := InitWhitelist("./whitelists/ip_whitelist.txt", make(chan bool))

	if !reflect.DeepEqual(wl.list, ip_mapData) {
		t.Errorf("InitWhitelist() did not correctly update the list\n\treceived: %#v\n\texpected: %#v", wl.list, ip_mapData)
	}
}

// Checks if the whitelist updates when the whitelist file changes
func TestInitWhitelist_Change(t *testing.T) {
	// Reset whitelist file to default
	err := ioutil.WriteFile("./whitelists/change_test.txt", change_fileData1, 0644)
	if err != nil {
		jww.ERROR.Printf("Failed to write to file: %v", err)
	}

	time.Sleep(2 * time.Second)

	// Initialise whitelist
	updateFinished := make(chan bool)
	wl := InitWhitelist("./whitelists/change_test.txt", updateFinished)

	time.Sleep(2 * time.Second)

	// Check
	if !reflect.DeepEqual(wl.list, change_mapData1) {
		t.Errorf("UpdateWhitelist() did not correctly update the list when changed\n\treceived: %#v\n\texpected: %#v", wl.list, change_mapData1)
	}

	// Write new data to file
	err = ioutil.WriteFile(wl.file, change_fileData2, 0644)
	if err != nil {
		jww.ERROR.Printf("Failed to write to file: %v", err)
	}

	// Check
	<-updateFinished
	if !reflect.DeepEqual(wl.list, change_mapData2) {
		t.Errorf("UpdateWhitelist() did not correctly update the list when changed\n\treceived: %#v\n\texpected: %#v", wl.list, change_mapData2)
	}

	// Write new data to file
	err = ioutil.WriteFile(wl.file, change_fileData1, 0644)
	if err != nil {
		jww.ERROR.Printf("Failed to write to file: %v", err)
	}

	// Check
	<-updateFinished
	if !reflect.DeepEqual(wl.list, change_mapData1) {
		t.Errorf("UpdateWhitelist() did not correctly update the list when changed\n\treceived: %#v\n\texpected: %#v", wl.list, change_mapData1)
	}

	// Write new data to file
	err = ioutil.WriteFile(wl.file, change_fileData2, 0644)
	if err != nil {
		jww.ERROR.Printf("Failed to write to file: %v", err)
	}

	// Check
	<-updateFinished
	if !reflect.DeepEqual(wl.list, change_mapData2) {
		t.Errorf("UpdateWhitelist() did not correctly update the list when changed\n\treceived: %#v\n\texpected: %#v", wl.list, change_mapData2)
	}

	// Restore original file contents
	err = ioutil.WriteFile(wl.file, change_fileData1, 0644)
	if err != nil {
		jww.ERROR.Printf("Failed to write to file: %v", err)
	}

	// Check
	<-updateFinished
	if !reflect.DeepEqual(wl.list, change_mapData1) {
		t.Errorf("UpdateWhitelist() did not correctly update the list when changed\n\treceived: %#v\n\texpected: %#v", wl.list, change_mapData1)
	}
}

// Checks if the whitelist map is updated when UpdateWhitelist() is called.
func TestUpdateWhitelist(t *testing.T) {
	// Initialise values
	initMap := map[string]bool{"137": true, "152": true, "0": true, "255": true}
	filePath := "./whitelists/ip_whitelist.txt"
	wl := Whitelist{list: initMap, file: filePath}

	// Update list
	wl.UpdateWhitelist()

	// Check
	if !reflect.DeepEqual(wl.list, ip_mapData) {
		t.Errorf("UpdateWhitelist() did not correctly update the list\n\treceived: %#v\n\texpected: %#v", wl.list, ip_mapData)
	}
}

// Checks if the whitelist map is updated when UpdateWhitelist() is called
// multiple times on a changing whitelist file.
func TestUpdateWhitelist_Change(t *testing.T) {
	// Initialise values
	initMap := map[string]bool{"137": true, "152": true, "0": true, "255": true}
	filePath := "./whitelists/change_test.txt"
	wl := Whitelist{list: initMap, file: filePath}

	// Reset whitelist file to default
	err := ioutil.WriteFile(wl.file, change_fileData1, 0644)
	if err != nil {
		jww.ERROR.Printf("Failed to write to file: %v", err)
	}

	// Update list
	wl.UpdateWhitelist()

	// Check
	if !reflect.DeepEqual(wl.list, change_mapData1) {
		t.Errorf("UpdateWhitelist() did not correctly update the list when changed\n\treceived: %#v\n\texpected: %#v", wl.list, change_mapData1)
	}

	// Reset whitelist file to default
	err = ioutil.WriteFile(wl.file, change_fileData2, 0644)
	if err != nil {
		jww.ERROR.Printf("Failed to write to file: %v", err)
	}

	// Update list
	wl.UpdateWhitelist()

	// Check
	if !reflect.DeepEqual(wl.list, change_mapData2) {
		t.Errorf("UpdateWhitelist() did not correctly update the list when changed\n\treceived: %#v\n\texpected: %#v", wl.list, change_mapData2)
	}

	// Restore file contents
	err = ioutil.WriteFile(wl.file, change_fileData1, 0644)
	if err != nil {
		jww.ERROR.Printf("Failed to write to file: %v", err)
	}
}

// Tests that an empty whitelist file produces a map with no elements.
func TestUpdateWhitelist_Empty(t *testing.T) {
	// Initialise values
	initMap := map[string]bool{"137": true, "152": true, "0": true, "255": true}
	filePath := "./whitelists/empty_test.txt"
	wl := Whitelist{list: initMap, file: filePath}
	testList := map[string]bool{}

	// Update list
	wl.UpdateWhitelist()

	// Check
	if !reflect.DeepEqual(wl.list, testList) {
		t.Errorf("UpdateWhitelist() did not correctly update the list retrieved from an empty file\n\treceived: %#v\n\texpected: %#v", wl.list, testList)
	}
}

// Tests that a whitelist file with only one line produces a map with only one
// element.
func TestUpdateWhitelist_NoNewline(t *testing.T) {
	// Initialise values
	initMap := map[string]bool{"137": true, "152": true, "0": true, "255": true}
	filePath := "./whitelists/no_newline_test.txt"
	wl := Whitelist{list: initMap, file: filePath}

	// Update list
	wl.UpdateWhitelist()

	// Check
	if !reflect.DeepEqual(wl.list, noNewLine_mapData) {
		t.Errorf("UpdateWhitelist() did not correctly update the list retrieved from a file with no line breaks\n\treceived: %#v\n\texpected: %#v", wl.list, noNewLine_mapData)
	}
}

// Tests if the read lock correctly blocks write attempts to the map.
func TestUpdateWhitelist_ReadLock(t *testing.T) {
	// Initialise values
	initMap := map[string]bool{"137": true, "152": true, "0": true, "255": true}
	filePath := "./whitelists/ip_whitelist.txt"
	wl := Whitelist{list: initMap, file: filePath}

	result := make(chan bool)

	wl.RLock()

	go func() {
		wl.UpdateWhitelist()
		result <- true
	}()

	select {
	case <-result:
		t.Errorf("UpdateWhitelist() did not correctly lock the thread")
	case <-time.After(2 * time.Second):
		return
	}
}

// Tests if the file parser parses IP whitelist correctly.
func TestWhitelistFileParse(t *testing.T) {
	fileList, err := WhitelistFileParse("./whitelists/ip_whitelist.txt")

	if err != nil {
		t.Errorf("WhitelistFileParse() produced an unexpected error\n\treceived: %#v\n\texpected: %#v", err, nil)
	}

	if !reflect.DeepEqual(fileList, ip_fileData) {
		t.Errorf("WhitelistFileParse() did not read the correct values from file\n\treceived: %#v\n\texpected: %#v", fileList, ip_fileData)
	}
}

// Tests if the file parser parses an empty file correctly without error.
func TestWhitelistFileParse_Empty(t *testing.T) {
	fileList, err := WhitelistFileParse("./whitelists/empty_test.txt")

	if err != nil {
		t.Errorf("WhitelistFileParse() produced an unexpected error\n\treceived: %#v\n\texpected: %#v", err, nil)
	}

	if !reflect.DeepEqual(fileList, []string{}) {
		t.Errorf("WhitelistFileParse() did not read the correct values from empty file\n\treceived: %#v\n\texpected: %#v", fileList, []string{})
	}
}

// Tests if the file parser parses a single line file correctly into string
// slice with one element.
func TestWhitelistFileParse_NoNewline(t *testing.T) {
	fileList, err := WhitelistFileParse("./whitelists/no_newline_test.txt")

	if err != nil {
		t.Errorf("WhitelistFileParse() produced an unexpected error\n\treceived: %#v\n\texpected: %#v", err, nil)
	}

	if !reflect.DeepEqual(fileList, noNewLine_fileData) {
		t.Errorf("WhitelistFileParse() did not read the correct values from no new line file\n\treceived: %#v\n\texpected: %#v", fileList, noNewLine_fileData)
	}
}

// Tests if the file parser throws and error when no file is present.
func TestWhitelistFileParse_NoFile(t *testing.T) {
	fileList, err := WhitelistFileParse("./whitelists/no_name.txt")

	if err == nil {
		t.Errorf("WhitelistFileParse() did not produce an error when it should have\n\treceived: %#v\n\texpected: %#v", err, errors.New("some error"))
	}

	if !reflect.DeepEqual(fileList, []string{}) {
		t.Errorf("WhitelistFileParse() did not read the correct values from file\n\treceived: %#v\n\texpected: %#v", fileList, []string{})
	}
}

// Tests if the watcher correctly updates the whitelist map when the whitelist
// file is changed.
func TestWhitelistWatcher(t *testing.T) {

	// Reset whitelist file to default
	err := ioutil.WriteFile("./whitelists/change_test.txt", change_fileData1, 0644)
	if err != nil {
		jww.ERROR.Printf("Failed to write to file: %v", err)
	}

	// Initialise values
	initMap := map[string]bool{"137": true, "152": true, "0": true, "255": true}
	filePath := "./whitelists/change_test.txt"
	wl := Whitelist{list: initMap, file: filePath}

	time.Sleep(4 * time.Second)

	// Run watcher in its own thread
	updateFinished := make(chan bool)
	go wl.WhitelistWatcher(updateFinished)

	time.Sleep(1 * time.Second)

	// Write new data to file
	err = ioutil.WriteFile(wl.file, change_fileData2, 0644)
	if err != nil {
		jww.ERROR.Printf("Failed to write to file: %v", err)
	}

	// Check if the list was updated
	<-updateFinished
	if !reflect.DeepEqual(wl.list, change_mapData2) {
		t.Errorf("WhitelistWatcher() did not get updated file correctly\n\tfile:     %v\n\treceived: %#v\n\texpected: %#v", filePath, wl.list, change_mapData2)
	}

	// Write new data to file
	err = ioutil.WriteFile(wl.file, change_fileData1, 0644)
	if err != nil {
		jww.ERROR.Printf("Failed to write to file: %v", err)
	}

	// Check if the list was updated
	<-updateFinished
	if !reflect.DeepEqual(wl.list, change_mapData1) {
		t.Errorf("WhitelistWatcher() did not get updated file correctly\n\tfile:     %v\n\treceived: %#v\n\texpected: %#v", filePath, wl.list, change_mapData1)
	}

	// Write new data to file
	err = ioutil.WriteFile(wl.file, change_fileData2, 0644)
	if err != nil {
		jww.ERROR.Printf("Failed to write to file: %v", err)
	}

	// Check if the list was updated
	<-updateFinished
	if !reflect.DeepEqual(wl.list, change_mapData2) {
		t.Errorf("WhitelistWatcher() did not get updated file correctly\n\tfile:     %v\n\treceived: %#v\n\texpected: %#v", filePath, wl.list, change_mapData2)
	}

	// Restore file contents
	err = ioutil.WriteFile(wl.file, change_fileData1, 0644)
	if err != nil {
		jww.ERROR.Printf("Failed to write to file: %v", err)
	}

	time.Sleep(4 * time.Second)

	// Check if the list was updated
	<-updateFinished
	if !reflect.DeepEqual(wl.list, change_mapData1) {
		t.Errorf("WhitelistWatcher() did not get updated file correctly\n\tfile:     %v\n\treceived: %#v\n\texpected: %#v", filePath, wl.list, change_mapData1)
	}
}

// Tests that the list is no longer updated after the watcher thread is closed.
func TestWhitelistWatcherClose(t *testing.T) {

	// Reset whitelist file to default
	err := ioutil.WriteFile("./whitelists/change_test.txt", change_fileData1, 0644)
	if err != nil {
		jww.ERROR.Printf("Failed to write to file: %v", err)
	}

	// Initialise values
	initMap := map[string]bool{"137": true, "152": true, "0": true, "255": true}
	filePath := "./whitelists/change_test.txt"
	wl := Whitelist{list: initMap, file: filePath}

	time.Sleep(4 * time.Second)

	// Run watcher in its own thread
	updateFinished := make(chan bool)
	go wl.WhitelistWatcher(updateFinished)

	time.Sleep(1 * time.Second)

	// Write new data to file
	err = ioutil.WriteFile(wl.file, change_fileData2, 0644)
	if err != nil {
		jww.ERROR.Printf("Failed to write to file: %v", err)
	}

	// Check if the list was updated
	<-updateFinished
	if !reflect.DeepEqual(wl.list, change_mapData2) {
		t.Errorf("WhitelistWatcher() did not get updated file correctly\n\tfile:     %v\n\treceived: %#v\n\texpected: %#v", filePath, wl.list, change_mapData2)
	}

	ok := wl.WhitelistWatcherClose()
	if ok != true {
		t.Errorf("WhitelistWatcherClose() failed to close the watcher thread")
	}

	// Write new data to file
	err = ioutil.WriteFile(wl.file, change_fileData1, 0644)
	if err != nil {
		jww.ERROR.Printf("Failed to write to file: %v", err)
	}

	time.Sleep(4 * time.Second)

	if !reflect.DeepEqual(wl.list, change_mapData2) {
		t.Errorf("WhitelistWatcherClose() failed to close the watcher thread because the whitelist was updated\n\tfile:     %v\n\treceived: %#v\n\texpected: %#v", filePath, wl.list, change_mapData2)
	}
}

// Tests that Exists() returns elements that exist and does not return elements
// that do not exist.
func TestExists(t *testing.T) {
	// Initialise values
	wl := InitWhitelist("./whitelists/ip_whitelist.txt", make(chan bool))
	var exists bool

	exists = wl.Exists(ip_fileData[0])
	if !exists {
		t.Errorf("Exists() failed to find the key %#v, which should exist in the map\n\treceived: %v\n\texpected: %v", ip_fileData[0], exists, true)
	}

	exists = wl.Exists(ip_fileData[5])
	if !exists {
		t.Errorf("Exists() failed to find the key %#v, which should exist in the map\n\treceived: %v\n\texpected: %v", ip_fileData[5], exists, true)
	}

	exists = wl.Exists(ip_fileData[21])
	if !exists {
		t.Errorf("Exists() failed to find the key %#v, which should exist in the map\n\treceived: %v\n\texpected: %v", ip_fileData[21], exists, true)
	}

	exists = wl.Exists("A")
	if exists {
		t.Errorf("Exists() found the key %#v, which should NOT exist in the map\n\treceived: %v\n\texpected: %v", "A", exists, true)
	}

	exists = wl.Exists("159.8.41.13")
	if exists {
		t.Errorf("Exists() found the key %#v, which should NOT exist in the map\n\treceived: %v\n\texpected: %v", "159.8.41.13", exists, true)
	}

	exists = wl.Exists("")
	if exists {
		t.Errorf("Exists() found the key %#v, which should NOT exist in the map\n\treceived: %v\n\texpected: %v", "", exists, true)
	}
}

// Tests if Exists() returns false on an empty list.
func TestExists_Empty(t *testing.T) {
	// Initialise values
	wl := InitWhitelist("./whitelists/empty_test.txt", make(chan bool))
	var exists bool

	exists = wl.Exists("")
	if exists {
		t.Errorf("Exists() found the key %#v, which should NOT exist in the map\n\treceived: %v\n\texpected: %v", "", exists, true)
	}

	exists = wl.Exists("hello")
	if exists {
		t.Errorf("Exists() found the key %v, which should NOT exist in the map\n\treceived: %v\n\texpected: %v", "hello", exists, true)
	}
}

// Tests the thread lock when reading from the map.
func TestExists_WriteLock(t *testing.T) {
	wl := InitWhitelist("./whitelists/ip_whitelist.txt", make(chan bool))

	result := make(chan bool)

	wl.Lock()

	go func() {
		wl.Exists("159.8.41.131")
		result <- true
	}()

	select {
	case <-result:
		t.Errorf("Exists() did not correctly lock the thread")
	case <-time.After(2 * time.Second):
		return
	}
}

// Tests if new lines from Mac and Windows are correctly transformed into UNIX
// new lines.
func TestNormalizeNewlines(t *testing.T) {
	data1 := []byte{0, 54, 255, 13, 10, 128, 5}
	data2 := []byte{0, 54, 255, 13, 128, 5}
	data3 := []byte{0, 54, 255, 10, 128, 5}
	dataExpect := []byte{0, 54, 255, 10, 128, 5}

	nData := normalizeNewlines(data1)
	if !reflect.DeepEqual(nData, dataExpect) {
		t.Errorf("normalizeNewlines() did not replace the Windows new line (\r\n) correctly\n\treceived: %#v\n\texpected: %#v", nData, dataExpect)
	}

	nData = normalizeNewlines(data2)
	if !reflect.DeepEqual(nData, dataExpect) {
		t.Errorf("normalizeNewlines() did not replace the Mac new line (\r) correctly\n\treceived: %#v\n\texpected: %#v", nData, dataExpect)
	}

	nData = normalizeNewlines(data3)
	if !reflect.DeepEqual(nData, dataExpect) {
		t.Errorf("normalizeNewlines() replace new line icorrectly when none should be replaced\n\treceived: %#v\n\texpected: %#v", nData, dataExpect)
	}
}
