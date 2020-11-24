////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// Handles low level database control and interfaces

package storage

import (
	"fmt"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/xx_network/primitives/id"
	"sync"
	"time"
)

// Interface declaration for storage methods
type database interface {
	GetClient(id *id.ID) (*Client, error)
	InsertClient(client *Client) error
	UpsertClient(client *Client) error

	GetRound(id id.Round) (*Round, error)
	GetRounds(ids []id.Round) ([]*Round, error)
	UpsertRound(round *Round) error

	countMixedMessagesByRound(roundId id.Round) (uint64, error)
	getMixedMessages(recipientId *id.ID, roundId id.Round) ([]*MixedMessage, error)
	InsertMixedMessages(msgs []*MixedMessage) error
	DeleteMixedMessageByRound(roundId id.Round) error

	GetEpoch(id uint64) (*Epoch, error)
	GetLatestEpoch() (*Epoch, error)
	InsertEpoch(roundId id.Round) (*Epoch, error)

	getBloomFilters(recipientId *id.ID) ([]*BloomFilter, error)
	UpsertBloomFilter(filter *BloomFilter) error
	DeleteBloomFilterByEpoch(epochId uint64) error
}

// Struct implementing the database Interface with an underlying DB
type DatabaseImpl struct {
	db *gorm.DB // Stored database connection
}

// Struct implementing the database Interface with an underlying Map
type MapImpl struct {
	clients       map[id.ID]*Client
	rounds        map[id.Round]*Round
	mixedMessages MixedMessageMap
	bloomFilters  BloomFilterMap
	epochs        EpochMap
	sync.RWMutex
}

// MixedMessageMap contains a list of MixedMessage sorted into two maps so that
// they can key on RoundId and RecipientId. All messages are stored by their
// unique ID.
type MixedMessageMap struct {
	RoundId      map[id.Round]map[id.ID]map[uint64]*MixedMessage
	RecipientId  map[id.ID]map[id.Round]map[uint64]*MixedMessage
	RoundIdCount map[id.Round]uint64
	IdTrack      uint64
	sync.RWMutex
}

// BloomFilterMap contains a list of BloomFilter sorted in two maps so that they
// can key on RecipientId and EpochId.
type BloomFilterMap struct {
	RecipientId map[id.ID]map[uint64]*BloomFilter
	EpochId     map[uint64]map[id.ID]*BloomFilter
	sync.RWMutex
}

// EpochMap contains a map of Epoch keyed on their ID. Also tracks incrementing
// of new IDs, and the latest Epoch in the map.
type EpochMap struct {
	M       map[uint64]*Epoch
	IdTrack uint64
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

// Represents an Epoch that is associated with each BloomFilter
// Used to determine a time period during which a set of Filters were created
type Epoch struct {
	Id          uint64    `gorm:"primary_key;AUTO_INCREMENT:true"`
	RoundId     uint64    `gorm:"NOT NULL"` // Explicitly not a FK, a Round may be deleted
	DateCreated time.Time `gorm:"NOT NULL"`

	BloomFilters []BloomFilter `gorm:"foreignkey:EpochId;association_foreignkey:Id"`
}

// Represents a Client's BloomFilter
type BloomFilter struct {
	RecipientId []byte `gorm:"primary_key;"`
	EpochId     uint64 `gorm:"primary_key;type:bigint REFERENCES epochs(Id)"`
	Filter      []byte `gorm:"NOT NULL"`
}

// Used to force correct pluralization of Epoch table name
func (Epoch) TableName() string {
	return "epochs"
}

// Represents a MixedMessage and its contents
type MixedMessage struct {
	Id              uint64 `gorm:"primary_key;AUTO_INCREMENT:true"`
	RoundId         uint64 `gorm:"INDEX;NOT NULL"`
	RecipientId     []byte `gorm:"INDEX;NOT NULL"`
	MessageContents []byte `gorm:"NOT NULL"`
}

// Creates a new MixedMessage object with the given attributes
// NOTE: Do not modify the MixedMessage.Id attribute.
func NewMixedMessage(roundId id.Round, recipientId *id.ID, messageContentsA, messageContentsB []byte) *MixedMessage {
	return &MixedMessage{
		RoundId:         uint64(roundId),
		RecipientId:     recipientId.Marshal(),
		MessageContents: append(messageContentsA, messageContentsB...),
	}
}

// Return the separated message contents of the MixedMessage
func (m *MixedMessage) GetMessageContents() (messageContentsA, messageContentsB []byte) {
	splitPosition := len(m.MessageContents) / 2
	messageContentsA = m.MessageContents[:splitPosition]
	messageContentsB = m.MessageContents[splitPosition:]
	return
}

// Initialize the database interface with database backend
// Returns a database interface, close function, and error
func newDatabase(username, password, dbName, address,
	port string) (database, func() error, error) {

	var err error
	var db *gorm.DB
	// Connect to the database if the correct information is provided
	if address != "" && port != "" {
		// Create the database connection
		connectString := fmt.Sprintf(
			"host=%s port=%s user=%s dbname=%s sslmode=disable",
			address, port, username, dbName)
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
			clients: map[id.ID]*Client{},
			rounds:  map[id.Round]*Round{},

			mixedMessages: MixedMessageMap{
				RoundId:      map[id.Round]map[id.ID]map[uint64]*MixedMessage{},
				RecipientId:  map[id.ID]map[id.Round]map[uint64]*MixedMessage{},
				RoundIdCount: map[id.Round]uint64{},
				IdTrack:      0,
			},
			bloomFilters: BloomFilterMap{
				RecipientId: map[id.ID]map[uint64]*BloomFilter{},
				EpochId:     map[uint64]map[id.ID]*BloomFilter{},
			},
			epochs: EpochMap{
				M:       map[uint64]*Epoch{},
				IdTrack: 0,
			},
		}

		return database(mapImpl), func() error { return nil }, nil
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
		&Epoch{}, &BloomFilter{}}
	for _, model := range models {
		err = db.AutoMigrate(model).Error
		if err != nil {
			return database(&DatabaseImpl{}), func() error { return nil }, err
		}
	}

	// Build the interface
	di := &DatabaseImpl{
		db: db,
	}

	jww.INFO.Println("Database backend initialized successfully!")
	return database(di), db.Close, nil
}
