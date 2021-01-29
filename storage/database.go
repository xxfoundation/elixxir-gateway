////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// Handles low level database control and interfaces

package storage

import (
	"fmt"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"sync"
)

// Interface declaration for storage methods
type database interface {
	UpsertState(state *State) error
	GetStateValue(key string) (string, error)

	GetClient(id *id.ID) (*Client, error)
	InsertClient(client *Client) error
	UpsertClient(client *Client) error

	GetRound(id id.Round) (*Round, error)
	GetRounds(ids []id.Round) ([]*Round, error)
	UpsertRound(round *Round) error

	countMixedMessagesByRound(roundId id.Round) (uint64, error)
	getMixedMessages(recipientId *ephemeral.Id, roundId id.Round) ([]*MixedMessage, error)
	InsertMixedMessages(msgs []*MixedMessage) error
	DeleteMixedMessageByRound(roundId id.Round) error

	GetClientBloomFilters(recipientId *ephemeral.Id, startEpoch, endEpoch uint32) ([]*ClientBloomFilter, error)
	upsertClientBloomFilter(filter *ClientBloomFilter) error
	DeleteClientFiltersBeforeEpoch(epoch uint32) error
}

// Struct implementing the database Interface with an underlying DB
type DatabaseImpl struct {
	db *gorm.DB // Stored database connection
}

// Struct implementing the database Interface with an underlying Map
type MapImpl struct {
	states        map[string]string
	clients       map[id.ID]*Client
	rounds        map[id.Round]*Round
	mixedMessages MixedMessageMap
	bloomFilters  BloomFilterMap
	sync.RWMutex
}

// MixedMessageMap contains a list of MixedMessage sorted into two maps so that
// they can key on RoundId and RecipientId. All messages are stored by their
// unique ID.
type MixedMessageMap struct {
	RoundId      map[id.Round]map[int64]map[uint64]*MixedMessage
	RecipientId  map[int64]map[id.Round]map[uint64]*MixedMessage
	RoundIdCount map[id.Round]uint64
	IdTrack      uint64
	sync.RWMutex
}

// BloomFilterMap contains a list of ClientBloomFilter sorted in a map that can key on RecipientId.
type BloomFilterMap struct {
	RecipientId map[int64]*ClientBloomFilterList
	sync.RWMutex
}

type ClientBloomFilterList struct {
	list  []*ClientBloomFilter
	start uint32
}

// Key-Value store used for persisting Gateway State information
type State struct {
	Key   string `gorm:"primaryKey"`
	Value string `gorm:"not null"`
}

// Enumerates Keys in the State table
const (
	PeriodKey = "Period"
)

// Represents a Client and its associated keys
type Client struct {
	Id  []byte `gorm:"primaryKey"`
	Key []byte `gorm:"not null"`
}

// Represents a Round and its associated information
type Round struct {
	Id       uint64 `gorm:"primaryKey;autoIncrement:false"`
	UpdateId uint64 `gorm:"unique"`
	InfoBlob []byte

	Messages []MixedMessage `gorm:"foreignKey:RoundId"`
}

// Represents a ClientBloomFilter
type ClientBloomFilter struct {
	Epoch       uint32 `gorm:"primaryKey"`
	RecipientId int64  `gorm:"primaryKey"`
	FirstRound  uint64 `gorm:"not null"`
	RoundRange  uint32 `gorm:"not null"`
	Filter      []byte `gorm:"not null"`
}

// Represents a MixedMessage and its contents
type MixedMessage struct {
	Id              uint64 `gorm:"primaryKey;autoIncrement:true"`
	RoundId         uint64 `gorm:"index;not null;references rounds(id)"`
	RecipientId     int64  `gorm:"index;not null"`
	MessageContents []byte `gorm:"not null"`
}

// Creates a new MixedMessage object with the given attributes
// NOTE: Do not modify the MixedMessage.Id attribute.
func NewMixedMessage(roundId id.Round, recipientId *ephemeral.Id, messageContentsA, messageContentsB []byte) *MixedMessage {
	return &MixedMessage{
		RoundId:         uint64(roundId),
		RecipientId:     recipientId.Int64(),
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
	port string) (database, error) {

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
		db, err = gorm.Open(postgres.Open(connectString), &gorm.Config{
			Logger: logger.New(jww.TRACE, logger.Config{LogLevel: logger.Info}),
		})
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
			states:  map[string]string{},

			mixedMessages: MixedMessageMap{
				RoundId:      map[id.Round]map[int64]map[uint64]*MixedMessage{},
				RecipientId:  map[int64]map[id.Round]map[uint64]*MixedMessage{},
				RoundIdCount: map[id.Round]uint64{},
				IdTrack:      0,
			},
			bloomFilters: BloomFilterMap{
				RecipientId: map[int64]*ClientBloomFilterList{},
			},
		}

		return database(mapImpl), nil
	}

	// Initialize the database schema
	// WARNING: Order is important. Do not change without database testing
	models := []interface{}{&Client{}, &Round{}, &MixedMessage{}, &ClientBloomFilter{}, State{}}
	for _, model := range models {
		err = db.AutoMigrate(model)
		if err != nil {
			return database(&DatabaseImpl{}), err
		}
	}

	// Build the interface
	di := &DatabaseImpl{
		db: db,
	}

	jww.INFO.Println("Database backend initialized successfully!")
	return database(di), nil
}
