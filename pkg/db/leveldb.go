package db

import "github.com/syndtr/goleveldb/leveldb"

var db *LevelDB

type LevelDB struct {
	db   *leveldb.DB
	path string
}

func InitDB(path string) {
	if db != nil {
		return
	}

	if path == "" {
		panic("leveldb path is empty!!!")
	}

	var err error
	ldb, err := leveldb.OpenFile(path, nil)
	if err != nil {
		panic(err)
	}

	db = &LevelDB{
		db:   ldb,
		path: path,
	}

}

func GetDB() *LevelDB {
	return db
}

/*
保存
*/
func (this *LevelDB) Save(id []byte, bs *[]byte) error {

	//levedb保存相同的key，原来的key保存的数据不会删除，因此保存之前先删除原来的数据
	err := this.db.Delete(id, nil)
	if err != nil {
		return err
	}
	if bs == nil {
		err = this.db.Put(id, nil, nil)
	} else {
		err = this.db.Put(id, *bs, nil)
	}
	return err
}

/*
查找
*/
func (this *LevelDB) Find(txId []byte) (*[]byte, error) {
	value, err := this.db.Get(txId, nil)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

/*
删除
*/
func (this *LevelDB) Remove(id []byte) error {
	return this.db.Delete(id, nil)
}
