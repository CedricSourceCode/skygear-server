package db

import (
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/skygeario/skygear-server/pkg/core/config"
	"github.com/skygeario/skygear-server/pkg/core/errors"
)

type DatabaseConfiguration struct {
	MaxOpenConns          int `envconfig:"MAX_OPEN_CONNS" default:"30"`
	MaxIdleConns          int `envconfig:"MAX_IDLE_CONNS" default:"5"`
	ConnMaxLifetimeSecond int `envconfig:"CONN_MAX_LIFETIME_SECOND" default:"1800"`
}

type Pool interface {
	Open(tConfig config.TenantConfiguration) (*sqlx.DB, error)
	OpenURL(url string) (*sqlx.DB, error)
	Close() error
}

type poolImpl struct {
	closed     bool
	closeMutex sync.RWMutex

	cache      map[string]*sqlx.DB
	cacheMutex sync.RWMutex

	config DatabaseConfiguration
}

func NewPool(config DatabaseConfiguration) Pool {
	p := &poolImpl{cache: map[string]*sqlx.DB{}, config: config}
	return p
}

func (p *poolImpl) OpenURL(source string) (db *sqlx.DB, err error) {
	p.closeMutex.RLock()
	defer func() { p.closeMutex.RUnlock() }()
	if p.closed {
		return nil, errors.New("skydb: pool is closed")
	}

	p.cacheMutex.RLock()
	db, exists := p.cache[source]
	p.cacheMutex.RUnlock()

	if !exists {
		p.cacheMutex.Lock()
		db, exists = p.cache[source]
		if !exists {
			db, err = p.openPostgresDB(source)
			if err == nil {
				p.cache[source] = db
			}
		}
		p.cacheMutex.Unlock()
	}

	return
}

func (p *poolImpl) Open(tConfig config.TenantConfiguration) (*sqlx.DB, error) {
	return p.OpenURL(tConfig.DatabaseConfig.DatabaseURL)
}

func (p *poolImpl) Close() (err error) {
	p.closeMutex.Lock()
	defer func() { p.closeMutex.Unlock() }()

	p.closed = true
	for _, db := range p.cache {
		if closeErr := db.Close(); closeErr != nil {
			err = closeErr
		}
	}

	return
}

func (p *poolImpl) openPostgresDB(url string) (db *sqlx.DB, err error) {
	db, err = sqlx.Open("postgres", url)
	if err != nil {
		return
	}

	// TODO(pool): configurable / profile for good value?
	db.SetMaxOpenConns(p.config.MaxOpenConns)
	db.SetMaxIdleConns(p.config.MaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(p.config.ConnMaxLifetimeSecond) * time.Second)
	return
}
