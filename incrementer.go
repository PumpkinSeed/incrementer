package incrmntr

import (
	"errors"
	"sync"

	"github.com/couchbase/gocb"
)

// Incrmntr is the base interface of the library
type Incrmntr interface {
	Get(key string) (int64, error)
	Add(key string) (NullInt64, error)
	AddSafe(key string) (NullInt64, error)
	AddWithRollover(key string, rollover uint64) (NullInt64, error)
	AddSafeWithRollover(key string, rollover uint64) (NullInt64, error)
	SetBucket(opts BucketOpts)
	Close() error
}

type BucketOpts struct {
	OperationTimeout      NullTimeout
	BulkOperationTimeout  NullTimeout
	DurabilityTimeout     NullTimeout
	DurabilityPollTimeout NullTimeout
	ViewTimeout           NullTimeout
	N1qlTimeout           NullTimeout
	AnalyticsTimeout      NullTimeout
}

// Incrementer is the main struct stores the related data
// and implements the Incrmntr interface
type Incrementer struct {
	sync.Mutex

	bucket   *gocb.Bucket
	rollover uint64
	initial  int64
	inc      uint64
	ttl      uint32
}

// New creates a new handler which implements the Incrmntr and setup the buckets
func New(bucket *gocb.Bucket, rollover uint64, initial int64, inc uint64) (Incrmntr, error) {
	return &Incrementer{
		bucket:   bucket,
		rollover: rollover,
		initial:  initial,
		inc:      inc,
	}, nil
}

// Get the value of the given key
func (i *Incrementer) Get(key string) (int64, error) {
	var v interface{}
	_, err := i.bucket.Get(key, &v)

	return int64(v.(float64)), err
}

// AddWithRollover is do the increment on the specified key
// custom rollover on the key available
func (i *Incrementer) AddWithRollover(key string, rollover uint64) (NullInt64, error) {
	if i.bucket == nil {
		return nullInt64(), errors.New("error bucket is nil")
	}
	return i.add(key, rollover)
}

// AddSafeWithRollover do the increment on the specified key
// concurrency and lock safe increment
// custom rollover on the key available
func (i *Incrementer) AddSafeWithRollover(key string, rollover uint64) (NullInt64, error) {
	if i.bucket == nil {
		return nullInt64(), errors.New("error bucket is nil")
	}

	var value NullInt64
	var err error
	value, err = i.add(key, rollover)
	if err == gocb.ErrTmpFail {
		for {
			value, err = i.add(key, rollover)
			if err == nil {
				break
			}
		}
	}
	if err != gocb.ErrTmpFail && err != nil {
		return nullInt64(), err
	}

	return value, nil
}

// Add is do the increment on the specified key
func (i *Incrementer) Add(key string) (NullInt64, error) {
	if i.bucket == nil {
		return nullInt64(), errors.New("error bucket is nil")
	}
	return i.add(key, i.rollover)
}

// AddSafe do the increment on the specified key
// concurrency and lock safe increment
func (i *Incrementer) AddSafe(key string) (NullInt64, error) {
	if i.bucket == nil {
		return nullInt64(), errors.New("error bucket is nil")
	}

	var value NullInt64
	var err error
	value, err = i.add(key, i.rollover)
	if err == gocb.ErrTmpFail {
		for {
			value, err = i.add(key, i.rollover)
			if err == nil {
				break
			}
		}
	}
	if err != gocb.ErrTmpFail && err != nil {
		return nullInt64(), err
	}

	return value, nil
}

// Close the bucket
func (i *Incrementer) Close() error {
	err := i.bucket.Close()
	i.bucket = nil
	return err
}

// add handle the increment mechanism, rollover passed as
// parameter because there is functions with custom rollover
func (i *Incrementer) add(key string, rollover uint64) (NullInt64, error) {
	var err error

	// ---- initKey called first to ensure key will be ready for operation
	initHappened, err := i.initKey(key)
	if err != nil {
		return nullInt64(), err
	}
	if initHappened {
		return nullInt64(), nil
	}

	// ---- get the current value and lock the cas
	var current interface{}
	cas, err := i.bucket.GetAndLock(key, i.ttl, &current)
	if err != nil {
		return nullInt64(), err
	}

	// ---- do the exact increment mechanism
	newValue := current.(float64) + float64(i.inc)
	if newValue > float64(rollover) {
		newValue = float64(i.initial)
	}
	_, err = i.bucket.Replace(key, newValue, cas, 0)

	// https://developer.couchbase.com/documentation/server/3.x/developer/dev-guide-3.0/lock-items.html

	return nullInt64From(int64(newValue)), err
}

// initKey do the key initialze process, it's means
// if the key not found, call the Counter which creates it
func (i *Incrementer) initKey(key string) (bool, error) {
	i.Lock()
	defer i.Unlock()

	// ---- v stores the value of the key
	var v interface{}

	// ---- is a flag, shows any action happened
	var happened = false

	// ---- check key is exists, if not create it
	_, err := i.bucket.Get(key, &v)
	if err == gocb.ErrKeyNotFound {
		i.bucket.Counter(key, i.initial, i.initial, 0)
		happened = true
	} else {
		return false, err
	}

	return happened, nil
}

func (i *Incrementer) SetBucket(opts BucketOpts) {
	if opts.OperationTimeout.valid {
		i.bucket.SetOperationTimeout(opts.OperationTimeout.Value)
	}
	if opts.BulkOperationTimeout.valid {
		i.bucket.SetBulkOperationTimeout(opts.BulkOperationTimeout.Value)
	}
	if opts.DurabilityTimeout.valid {
		i.bucket.SetDurabilityTimeout(opts.DurabilityTimeout.Value)
	}
	if opts.DurabilityPollTimeout.valid {
		i.bucket.SetDurabilityPollTimeout(opts.DurabilityPollTimeout.Value)
	}
	if opts.ViewTimeout.valid {
		i.bucket.SetViewTimeout(opts.ViewTimeout.Value)
	}
	if opts.N1qlTimeout.valid {
		i.bucket.SetN1qlTimeout(opts.N1qlTimeout.Value)
	}
	if opts.AnalyticsTimeout.valid {
		i.bucket.SetAnalyticsTimeout(opts.AnalyticsTimeout.Value)
	}
}
