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

	GetRound(id id.Round) (*Round, error)
	GetRounds(ids []id.Round) ([]*Round, error)
	UpsertRound(round *Round) error

	GetMixedMessages(recipientId *id.ID, roundId id.Round) ([]*MixedMessage, error)
	InsertMixedMessage(msg *MixedMessage) error
	DeleteMixedMessageByRound(roundId id.Round) error

	GetBloomFilters(clientId *id.ID) ([]*BloomFilter, error)
	InsertBloomFilter(filter *BloomFilter) error
	DeleteBloomFilter(id uint64) error

	GetEphemeralBloomFilters(recipientId *id.ID) ([]*EphemeralBloomFilter, error)
	InsertEphemeralBloomFilter(filter *EphemeralBloomFilter) error
	DeleteEphemeralBloomFilter(id uint64) error
}

// Struct implementing the database Interface with an underlying DB
type DatabaseImpl struct {
	db *gorm.DB // Stored database connection
}

// Struct implementing the database Interface with an underlying Map
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

//
type Epoch struct {
	Id          uint64    `gorm:"primary_key;AUTO_INCREMENT:false"`
	RoundId     uint64    `gorm:"NOT NULL"` // Explicitly not a FK
	DateCreated time.Time `gorm:"NOT NULL"`

	BloomFilters          []BloomFilter          `gorm:"foreignkey:EpochId;association_foreignkey:Id"`
	EphemeralBloomFilters []EphemeralBloomFilter `gorm:"foreignkey:EpochId;association_foreignkey:Id"`
}

// Represents a Client's BloomFilter
type BloomFilter struct {
	Id       uint64 `gorm:"primary_key;AUTO_INCREMENT:true"`
	ClientId []byte `gorm:"NOT NULL;INDEX;type:bytea REFERENCES clients(Id)"`
	Filter   []byte `gorm:"NOT NULL"`
	EpochId  uint64 `gorm:"NOT NULL;type:bigint REFERENCES epochs(Id)"`
}

// Represents an ephemeral Client's temporary BloomFilter
type EphemeralBloomFilter struct {
	Id          uint64 `gorm:"primary_key;AUTO_INCREMENT:true"`
	RecipientId []byte `gorm:"NOT NULL"`
	Filter      []byte `gorm:"NOT NULL"`
	EpochId     uint64 `gorm:"NOT NULL;type:bigint REFERENCES epochs(Id)"`
}

// Represents a MixedMessage and its contents
type MixedMessage struct {
	Id              uint64 `gorm:"primary_key;AUTO_INCREMENT:true"`
	RoundId         uint64 `gorm:"INDEX;NOT NULL"`
	RecipientId     []byte `gorm:"INDEX;NOT NULL"`
	MessageContents []byte `gorm:"NOT NULL"`
}

// Creates a new MixedMessage object with the given attributes
func NewMixedMessage(roundId *id.Round, recipientId *id.ID, messageContentsA, messageContentsB []byte) *MixedMessage {
	return &MixedMessage{
		RoundId:         uint64(*roundId),
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
// Returns a database interface, Close function, and error
func NewDatabase(username, password, dbName, address,
	port string) (database, func() error, error) {

	var err error
	var db *gorm.DB
	//connect to the database if the correct information is provided
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
			jww.WARN.Printf("database backend connection information not provided")
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
		&BloomFilter{}, &EphemeralBloomFilter{}}
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

	jww.INFO.Println("database backend initialized successfully!")
	return database(di), db.Close, nil
}
