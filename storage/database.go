////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

// Handles low level database control and interfaces

package storage

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"sync"
	"time"
)

// Interface declaration for storage methods
type database interface {
	UpsertState(state *State) error
	GetStateValue(key string) (string, error)

	GetClient(id *id.ID) (*Client, error)
	UpsertClient(client *Client) error

	GetRound(id id.Round) (*Round, error)
	UpsertRound(round *Round) error
	deleteRound(ts time.Time) error

	countMixedMessagesByRound(roundId id.Round) (uint64, bool, error)
	getMixedMessages(recipientId ephemeral.Id, roundId id.Round) ([]*MixedMessage, error)
	InsertMixedMessages(cr *ClientRound) error
	deleteMixedMessages(ts time.Time) error

	GetClientBloomFilters(recipientId ephemeral.Id, startEpoch, endEpoch uint32) ([]*ClientBloomFilter, error)
	upsertClientBloomFilter(filter *ClientBloomFilter) error
	DeleteClientFiltersBeforeEpoch(epoch uint32) error

	// TODO: Currently not used. May want to remove.
	GetLowestBloomRound() (uint64, error)
	GetRounds(ids []id.Round) ([]*Round, error)
}

// DatabaseImpl implements the database interface with an underlying DB
type DatabaseImpl struct {
	db *gorm.DB // Stored database connection
}

// MapImpl implements the database interface with an underlying Map
type MapImpl struct {
	states           map[string]string
	statesLock       sync.RWMutex
	clients          map[id.ID]*Client
	clientsLock      sync.RWMutex
	rounds           map[id.Round]*Round
	roundsLock       sync.RWMutex
	clientRounds     map[uint64]*ClientRound
	clientRoundsLock sync.RWMutex
	mixedMessages    MixedMessageMap
	bloomFilters     BloomFilterMap
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
	primaryKey  *uint64
	sync.RWMutex
}

type ClientBloomFilterList struct {
	list  [][]*ClientBloomFilter
	start uint32
}

// State is a Key-Value store used for persisting Gateway information
type State struct {
	Key   string `gorm:"primaryKey"`
	Value string `gorm:"not null"`
}

// Enumerates various Keys in the State table.
const (
	PeriodKey           = "Period"
	LastUpdateKey       = "LastUpdateId"
	KnownRoundsKey      = "KnownRoundsV3"
	HttpsCertificateKey = "HttpsCertificate"
)

// Client and its associated keys.
type Client struct {
	Id  []byte `gorm:"primaryKey"`
	Key []byte `gorm:"not null"`
}

// Round represents the Round information that is relevant to Gateways.
type Round struct {
	Id          uint64 `gorm:"primaryKey;autoIncrement:false"`
	UpdateId    uint64 `gorm:"unique;not null"`
	InfoBlob    []byte
	LastUpdated time.Time `gorm:"index;not null"` // Timestamp of most recent Update
}

// ClientRound represents the Round information that is relevant to Clients.
type ClientRound struct {
	Id        uint64    `gorm:"primaryKey;autoIncrement:false"`
	Timestamp time.Time `gorm:"index;not null"` // Round Realtime timestamp

	Messages []MixedMessage `gorm:"foreignKey:RoundId;constraint:OnDelete:CASCADE"`
}

type ClientBloomFilter struct {
	Id uint64 `gorm:"primaryKey;autoIncrement:true"`
	// Pointer to enforce zero-value reading in ORM.
	// Additionally, we desire to make composite indexes on the more distinct column first.
	RecipientId *int64 `gorm:"index:idx_client_bloom_filters_recipient_id_epoch,priority:1;not null"`
	Epoch       uint32 `gorm:"index:idx_client_bloom_filters_recipient_id_epoch,priority:2;not null"`
	FirstRound  uint64 `gorm:"index;not null"`
	RoundRange  uint32 `gorm:"not null"`
	Filter      []byte `gorm:"not null"`
	Uses        uint32 `gorm:"not null;default:0"` // Keep track of how many times used
}

type MixedMessage struct {
	Id              uint64 `gorm:"primaryKey;autoIncrement:true"`
	RoundId         uint64 `gorm:"index;not null;references rounds(id)"`
	RecipientId     int64  `gorm:"index;not null"`
	MessageContents []byte `gorm:"not null"`
}

// NewMixedMessage creates a new MixedMessage object with the given attributes.
// NOTE: Do not modify the MixedMessage.Id attribute.
func NewMixedMessage(roundId id.Round, recipientId ephemeral.Id, messageContentsA, messageContentsB []byte) *MixedMessage {

	messageContents := make([]byte, len(messageContentsA)+len(messageContentsB))
	copy(messageContents[:len(messageContentsA)], messageContentsA)
	copy(messageContents[len(messageContentsA):], messageContentsB)

	return &MixedMessage{
		RoundId:         uint64(roundId),
		RecipientId:     recipientId.Int64(),
		MessageContents: messageContents,
	}
}

// GetMessageContents return the separated message contents of the MixedMessage.
func (m *MixedMessage) GetMessageContents() (messageContentsA, messageContentsB []byte) {
	splitPosition := len(m.MessageContents) / 2
	messageContentsA = m.MessageContents[:splitPosition]
	messageContentsB = m.MessageContents[splitPosition:]
	return
}

// Initialize the database interface with database backend
// Returns a database interface and error
func newDatabase(username, password, dbName, address,
	port string, devMode bool) (database, error) {

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

		var failReason string
		if err != nil {
			failReason = fmt.Sprintf("Unable to initialize database backend: %+v", err)
			jww.WARN.Printf(failReason)
		} else {
			failReason = "Database backend connection information not provided"
			jww.WARN.Printf(failReason)
		}

		if !devMode {
			jww.FATAL.Panicf("Gateway cannot run in production "+
				"without a database: %s", failReason)
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
			clientRounds: map[uint64]*ClientRound{},
		}

		return database(mapImpl), nil
	}

	// Get and configure the internal database ConnPool
	sqlDb, err := db.DB()
	if err != nil {
		return database(&DatabaseImpl{}), errors.Errorf(
			"Unable to configure database connection pool: %+v", err)
	}
	// SetMaxIdleConns sets the maximum number of connections in the idle connection pool.
	sqlDb.SetMaxIdleConns(10)
	// SetMaxOpenConns sets the maximum number of open connections to the Database.
	sqlDb.SetMaxOpenConns(50)
	// SetConnMaxLifetime sets the maximum amount of time a connection may be idle.
	sqlDb.SetConnMaxIdleTime(10 * time.Minute)
	// SetConnMaxLifetime sets the maximum amount of time a connection may be reused.
	sqlDb.SetConnMaxLifetime(12 * time.Hour)

	// Ensure database structure is up-to-date.
	err = migrate(db)
	if err != nil {
		return database(&DatabaseImpl{}), errors.Errorf(
			"Failed to migrate database: %+v", err)
	}

	// Build the interface
	di := &DatabaseImpl{
		db: db,
	}

	jww.INFO.Println("Database backend initialized successfully!")
	return database(di), nil
}

// migrate is a basic database structure migrator.
func migrate(db *gorm.DB) error {
	migrateTimestamp := time.Now()

	// Perform automatic migrations of basic table structure.
	// WARNING: Order is important. Do not change without database testing.
	err := db.AutoMigrate(&Client{}, &Round{}, &ClientRound{},
		&MixedMessage{}, &ClientBloomFilter{}, State{})
	if err != nil {
		return err
	}

	// Determine the current version of the database via structural checks.
	currentVersion := 0
	columns, err := db.Migrator().ColumnTypes(&ClientBloomFilter{})
	if err != nil {
		return err
	}
	for _, column := range columns {
		if isPrimaryKey, _ := column.PrimaryKey(); column.Name() == "id" && isPrimaryKey {
			currentVersion = 1
			break
		}
	}
	if !db.Migrator().HasIndex(ClientBloomFilter{}, "idx_client_bloom_filters_recipient_id") {
		currentVersion = 2
	}

	jww.INFO.Printf("Current database version: v%d", currentVersion)

	// Perform any required manual migrations.
	if minVersion := 1; currentVersion < minVersion {
		jww.INFO.Printf("Performing database migration from v%d -> v%d",
			currentVersion, minVersion)
		ctx, cancel := context.WithTimeout(context.Background(), dbTimeout*5)
		err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			err := tx.Exec("ALTER TABLE client_bloom_filters DROP CONSTRAINT client_bloom_filters_pkey;").Error
			if err != nil {
				return err
			}

			// Commit
			return tx.Exec("ALTER TABLE client_bloom_filters ADD PRIMARY KEY (id);").Error
		})
		cancel()
		if err != nil {
			return err
		}
		currentVersion = minVersion
	}
	if minVersion := 2; currentVersion < minVersion {
		jww.INFO.Printf("Performing database migration from v%d -> v%d",
			currentVersion, minVersion)
		ctx, cancel := context.WithTimeout(context.Background(), dbTimeout*5)
		err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			err := tx.Migrator().DropIndex(ClientBloomFilter{}, "idx_client_bloom_filters_epoch")
			if err != nil {
				return err
			}

			// Commit
			return tx.Migrator().DropIndex(ClientBloomFilter{}, "idx_client_bloom_filters_recipient_id")
		})
		cancel()
		if err != nil {
			return err
		}
		currentVersion = minVersion
	}

	jww.DEBUG.Printf("Database initialization took %s",
		time.Now().Sub(migrateTimestamp).String())
	return nil
}
