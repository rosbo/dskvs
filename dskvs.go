// dskvs is a key value store.  In this store, there are two level or
// mapping.  The store is organized in collections that hold members.
// Each member is represented by a page.  Each collection is represented
// by a map of members.
//
// dskvs addresses members using a 'collection/member' convention.
//
// To start using dskvs, create a Store object with dskvs.NewStore,
// specifying where in the current filesystem to save the Store's files.
// Then, call store.Load to load any pre-existing collections and/or
// members into your store instance.  When you're done using the store,
// call store.Close, which finishes writing the last updates
package dskvs

import (
	"sync"
)

const collKeySep = "/"

var (
	existLock     sync.RWMutex
	existingStore map[string]bool
	dirtyPages    chan *page
	toDelete      chan *member
)

func init() {
	existingStore = make(map[string]bool)
	dirtyPages = make(chan *page)
	toDelete = make(chan *member)
}

type Store struct {
	isLoaded    bool
	storagePath string
	coll        *collections
}

/*
	Meta operations on Store
*/

// Instantiate a new store reading from the specified path
func NewStore(path string) (*Store, error) {

	if !isValidPath(path) {
		return nil, errorPathInvalid(path)
	}

	return &Store{
		false,            // isLoaded
		expandPath(path), // storagePath
		newCollections(), // collections
	}, nil
}

// This call will block for disk IO.
// Loads the files in memory.
func (s *Store) Load() error {
	existLock.RLock()
	exists := existingStore[s.storagePath]
	existLock.RUnlock()

	if exists {
		return errorPathInUse(s.storagePath)
	}

	existLock.Lock()
	existingStore[s.storagePath] = true
	existLock.Unlock()

	// TODO scan the path for files, load them in memory
	return nil
}

// This call will block for disk IO.
// Finish writing dirty updates and close all the files. Report any
// error occuring doing so.
func (s *Store) Close() error {
	if !s.isLoaded {
		return errorStoreNotLoaded(s)
	}
	existLock.Lock()
	delete(existingStore, s.storagePath)
	existLock.Unlock()

	// TODO wait for the janitor to finish writing

	return nil
}

/*
	Storage operations
*/

func (s *Store) Get(fullKey string) (*string, error) {
	if !s.isLoaded {
		return nil, errorStoreNotLoaded(s)
	}

	if err := checkKeyValid(fullKey); err != nil {
		return nil, err
	}
	isColl := isCollectionKey(fullKey)

	if isColl {
		return nil, errorGetIsColl(fullKey)
	}

	coll, key, err := splitKeys(fullKey)
	if err != nil {
		return nil, err
	}

	return s.coll.get(coll, key)
}

// Gets all the members' value in the collection `coll`.
func (s *Store) GetAll(coll string) ([]*string, error) {
	if !s.isLoaded {
		return nil, errorStoreNotLoaded(s)
	}

	if err := checkKeyValid(coll); err != nil {
		return nil, err
	}
	isColl := isCollectionKey(coll)

	if !isColl {
		return nil, errorGetAllIsNotColl(coll)
	}

	return s.coll.getCollection(coll)
}

// Puts the given value into the key location.  `fullKey` should be a
// member,  not a collection.  There is no `PutAll` version of this
// call.  If you wish to add a collection all at once, iterate over your
// collection and call `Put` on each member.
func (s *Store) Put(fullKey string, value *string) error {
	if !s.isLoaded {
		return errorStoreNotLoaded(s)
	}

	if err := checkKeyValid(fullKey); err != nil {
		return err
	}
	isColl := isCollectionKey(fullKey)

	if isColl {
		return errorPutIsColl(fullKey, *value)
	}

	coll, key, err := splitKeys(fullKey)
	if err != nil {
		return err
	}

	return s.coll.put(coll, key, value)
}

// Deletes member with `fullKey` from the storage.
func (s *Store) Delete(fullKey string) error {
	if !s.isLoaded {
		return errorStoreNotLoaded(s)
	}

	if err := checkKeyValid(fullKey); err != nil {
		return err
	}
	isColl := isCollectionKey(fullKey)

	if isColl {
		return errorDeleteIsColl(fullKey)
	}

	coll, key, err := splitKeys(fullKey)
	if err != nil {
		return err
	}

	return s.coll.deleteKey(coll, key)
}

// Deletes all the members in collection `coll`
func (s *Store) DeleteAll(coll string) error {
	if !s.isLoaded {
		return errorStoreNotLoaded(s)
	}

	if err := checkKeyValid(coll); err != nil {
		return err
	}
	isColl := isCollectionKey(coll)

	if !isColl {
		return errorDeleteAllIsNotColl(coll)
	}

	return s.coll.deleteCollection(coll)
}
