package rateLimiting

import (
	"bytes"
	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"time"
)

type Whitelist struct {
	list         map[string]bool   // Contains the list of keys
	file         string            // Absolute URL for the whitelist file
	watcher      *fsnotify.Watcher // Watches for file changes
	sync.RWMutex                   // Only allows one writer at a time
}

// HACK HACK HACK HACK
// create the whitelist file from hardcoded data so we can read the whitelist file
func CreateWhitelistFile(filePath string, whitelist []string) error {
	f, err := os.Create(filePath)
	if err != nil {
		return errors.Errorf("ERROR failed to create whitelist file %s: %+v", filePath, err)
	}

	for i := 0; i < len(whitelist); i++ {
		_, err = f.WriteString(whitelist[i] + "\n")
		if err != nil {
			return errors.Errorf("ERROR failed to write value %s to whitelist file: %+v", whitelist[i], err)
		}
	}

	err = f.Close()
	if err != nil {
		return errors.Errorf("ERROR failed to close whitelist file: %+v", err)
	}
	return nil
}

// Initialises a map for the whitelist with the specified keys. The
// updateFinished channel receives a value when the map finises updating.
func InitWhitelist(filePath string, updateFinished chan bool) *Whitelist {
	wl := Whitelist{
		list: make(map[string]bool),
		file: filePath,
	}

	// Update the whitelist from the specified file
	wl.UpdateWhitelist()

	// Start watching the whitelist file for changes
	go wl.WhitelistWatcher(updateFinished)

	return &wl
}

// Initialises a map for the whitelist with the specified keys.
func (wl *Whitelist) UpdateWhitelist() {
	// Get list of strings from the whitelist file
	list, err := WhitelistFileParse(wl.file)

	// If the file was read successfully, update the list
	if err == nil {
		// Enable write lock while writing to the map
		wl.Lock()

		// Reset the whitelist to empty
		wl.list = make(map[string]bool)

		// Add all the keys to the map
		for _, key := range list {
			wl.list[key] = true
		}

		// Disable write lock when writing is done
		wl.Unlock()
	}
}

// Parses the given file and stores each value in a slice. Returns the slice and
// an error. The file is expected to have value separated by new lines.
func WhitelistFileParse(filePath string) ([]string, error) {
	// Load file contents into memory
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		jww.ERROR.Printf("Failed to read file: %v", err)
		return []string{}, err
	}

	// Convert the data to string, trim whitespace, and normalize new lines
	dataStr := strings.TrimSpace(string(normalizeNewlines(data)))

	// Return empty slice if the file is empty or only contains whitespace
	if dataStr == "" {
		return []string{}, nil
	}

	// Split the data at new lines and place in slice
	return strings.Split(dataStr, "\n"), nil
}

// Watches the specified whitelist file for changes. When changes occur, the
// file is parsed and the new whitelist is loaded into the whitelist map.
func (wl *Whitelist) WhitelistWatcher(updateFinished chan bool) {
	var err error
	wl.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		jww.ERROR.Printf("Failed to start new file watcher: %s", err)
	}

	// Defer closing the watcher and checking for errors
	defer wl.WhitelistWatcherClose()

	done := make(chan bool)
	go func(updateFinished chan bool) {
		for {
			select {
			case event, ok := <-wl.watcher.Events:
				if !ok {
					return
				}
				jww.DEBUG.Printf("File watcher event: %v", event)

				if event.Op&fsnotify.Write == fsnotify.Write {
					jww.DEBUG.Printf("Watcher modified file: %v", event.Name)

					// Wait for write to end
					time.Sleep(5 * time.Second)

					// Update the whitelist from the new file
					wl.UpdateWhitelist()

					// Signify that the update is done
					if updateFinished != nil {
						updateFinished <- true
					}
				}

			case err, ok := <-wl.watcher.Errors:
				if !ok {
					return
				}
				jww.DEBUG.Printf("File watcher error: %v", err)
			}
		}
	}(updateFinished)

	// Add file watcher
	err = wl.watcher.Add(wl.file)
	if err != nil {
		jww.ERROR.Printf("Failed to add file watcher: %s", err)
	}

	<-done
}

// Removes the file watcher and closes the events channel. Returns true for a
// successful channel close and false otherwise.
func (wl *Whitelist) WhitelistWatcherClose() bool {
	err := wl.watcher.Close()

	if err != nil {
		jww.ERROR.Printf("Failed to close file watcher: %s", err)
		return false
	}

	return true
}

// Searches if the specified key exists in the whitelist. Returns true if it
// exists and false otherwise.
func (wl *Whitelist) Exists(key string) bool {
	// Enable read lock while reading from the map
	wl.RLock()

	// Check if the key exists in the map
	_, ok := wl.list[key]

	// Disable read lock when reading is done
	wl.RUnlock()

	return ok
}

// Normalizes \r\n (Windows) and \r (Mac) into \n (UNIX).
func normalizeNewlines(d []byte) []byte {
	// Replace CR LF \r\n (Windows) with LF \n (UNIX)
	d = bytes.Replace(d, []byte{13, 10}, []byte{10}, -1)

	// Replace CF \r (Mac) with LF \n (UNIX)
	d = bytes.Replace(d, []byte{13}, []byte{10}, -1)

	return d
}
