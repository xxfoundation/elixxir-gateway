////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// Handles high level database control and interfaces

package storage

import (
	"fmt"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/elixxir/primitives/id"
	"sync"
	"time"
)

// Global variable for database interaction
var GatewayDB Storage

// Interface declaration for storage methods
type Storage interface {
	GetClient(id *id.ID) (*Client, error)
	InsertClient(client *Client) error

	GetRound(id *id.Round) (*Round, error)
	UpsertRound(round *Round) error

	GetMixedMessages(recipientId *id.ID, roundId *id.Round) ([]*MixedMessage, error)
	InsertMixedMessage(msg *MixedMessage) error
	DeleteMixedMessage(id uint64) error

	GetBloomFilters(clientId *id.ID) ([]*BloomFilter, error)
	InsertBloomFilter(filter *BloomFilter) error
	DeleteBloomFilter(id uint64) error

	GetEphemeralBloomFilters(recipientId *id.ID) ([]*EphemeralBloomFilter, error)
	InsertEphemeralBloomFilter(filter *EphemeralBloomFilter) error
	DeleteEphemeralBloomFilter(id uint64) error
}

// Struct implementing the Database Interface with an underlying DB
type DatabaseImpl struct {
	db *gorm.DB // Stored database connection
}

// Struct implementing the Database Interface with an underlying Map
type MapImpl struct {
	clients                    map[id.ID]*Client
	rounds                     map[id.Round]*Round
	mixedMessages              map[uint64]*MixedMessage
	bloomFilters               map[uint64]*BloomFilter
	ephemeralBloomFilters      map[uint64]*EphemeralBloomFilter
	mixedMessagesCount         uint64
	bloomFiltersCount          uint64
	ephemeralBloomFiltersCount uint64
	sync.RWMutex
}

// Represents a Client and its associated keys
type Client struct {
	Id  []byte `gorm:"primary_key"`
	Key []byte `gorm:"NOT NULL"`

	Filters []BloomFilter `gorm:"foreignkey:ClientId;association_foreignkey:Id"`
}

// Represents a Round and its associated information
type Round struct {
	Id       uint64 `gorm:"primary_key;AUTO_INCREMENT:false"`
	UpdateId uint64 `gorm:"UNIQUE"`
	InfoBlob []byte

	Messages []MixedMessage `gorm:"foreignkey:RoundId;association_foreignkey:Id"`
}

// Represents a MixedMessage and its contents
type MixedMessage struct {
	Id              uint64 `gorm:"primary_key;AUTO_INCREMENT:true"`
	RoundId         uint64 `gorm:"INDEX;NOT NULL"`
	RecipientId     []byte `gorm:"INDEX;NOT NULL"`
	MessageContents []byte `gorm:"NOT NULL"`
}

// Represents a Client's BloomFilter
type BloomFilter struct {
	Id          uint64    `gorm:"primary_key;AUTO_INCREMENT:true"`
	ClientId    []byte    `gorm:"INDEX;type:bytea REFERENCES clients(Id)"`
	Count       uint64    `gorm:"NOT NULL"`
	Filter      []byte    `gorm:"NOT NULL"`
	DateCreated time.Time `gorm:"NOT NULL"`
}

// Represents an ephemeral Client's temporary BloomFilter
type EphemeralBloomFilter struct {
	Id          uint64 `gorm:"primary_key;AUTO_INCREMENT:true"`
	RecipientId []byte `gorm:"NOT NULL"`
	Count       uint64 `gorm:"NOT NULL"`
	Filter      []byte `gorm:"NOT NULL"`
}

// Initialize the Database interface with database backend
// Returns a Storage interface, Close function, and error
func NewDatabase(username, password, database, address,
	port string) (Storage, func() error, error) {

	var err error
	var db *gorm.DB
	//connect to the database if the correct information is provided
	if address != "" && port != "" {
		// Create the database connection
		connectString := fmt.Sprintf(
			"host=%s port=%s user=%s dbname=%s sslmode=disable",
			address, port, username, database)
		// Handle empty database password
		if len(password) > 0 {
			connectString += fmt.Sprintf(" password=%s", password)
		}
		db, err = gorm.Open("postgres", connectString)
	}

	// Return the map-backend interface
	// in the event there is a database error or information is not provided
	if (address == "" || port == "") || err != nil {

		if err != nil {
			jww.WARN.Printf("Unable to initialize database backend: %+v", err)
		} else {
			jww.WARN.Printf("Database backend connection information not provided")
		}

		defer jww.INFO.Println("Map backend initialized successfully!")

		mapImpl := &MapImpl{
			clients:                    map[id.ID]*Client{},
			rounds:                     map[id.Round]*Round{},
			mixedMessages:              map[uint64]*MixedMessage{},
			bloomFilters:               map[uint64]*BloomFilter{},
			ephemeralBloomFilters:      map[uint64]*EphemeralBloomFilter{},
			mixedMessagesCount:         0,
			bloomFiltersCount:          0,
			ephemeralBloomFiltersCount: 0,
		}

		return Storage(mapImpl), func() error { return nil }, nil
	}

	// Initialize the database logger
	db.SetLogger(jww.TRACE)
	db.LogMode(true)

	// SetMaxIdleConns sets the maximum number of connections in the idle connection pool.
	db.DB().SetMaxIdleConns(10)
	// SetMaxOpenConns sets the maximum number of open connections to the database.
	db.DB().SetMaxOpenConns(100)
	// SetConnMaxLifetime sets the maximum amount of time a connection may be reused.
	db.DB().SetConnMaxLifetime(24 * time.Hour)

	// Initialize the database schema
	// WARNING: Order is important. Do not change without database testing
	models := []interface{}{&Client{}, &Round{}, &MixedMessage{},
		&BloomFilter{}, &EphemeralBloomFilter{}}
	for _, model := range models {
		err = db.AutoMigrate(model).Error
		if err != nil {
			return Storage(&DatabaseImpl{}), func() error { return nil }, err
		}
	}

	// Build the interface
	di := &DatabaseImpl{
		db: db,
	}

	jww.INFO.Println("Database backend initialized successfully!")
	return Storage(di), db.Close, nil
}
