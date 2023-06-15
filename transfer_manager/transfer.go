package transfer_manager

import (
	"bytes"
	"encoding/json"
	"errors"
	"file_transfer_manager/connTcpMessage"
	"file_transfer_manager/pkg/db"
	log "file_transfer_manager/pkg/zap"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"file_transfer_manager/pkg/utils"
)

const (
	Lenth                             int64 = 100 * 1024 //每次传输大小（200kb）
	ErrNum                            int   = 5          //传输失败重试次数 5次
	Second                            int64 = 10         //传输速度统计时间间隔 10秒
	recfilepath                             = "files"
	Transferlog_push_task_db_key            = "transfer_push_task_db_key"
	Transferlog_pull_task_db_key            = "transfer_pull_task_db_key"
	Transfer_push_task_id_max               = "transfer_push_task_id_max"
	Transfer_pull_task_id_max               = "transfer_pull_task_id_max"
	Transfer_task_expiration_interval       = 24 * 60 * 60 * time.Second        //任务有效期24小时
	Transfer_push_task_sharing_dirs         = "transfer_push_task_sharing_dirs" // 共享文件夹
	Transfer_pull_addr_white_list           = "transfer_pull_addr_white_list"   //有权限拉取文件的地址白名单
	Transfer_pull_task_if_atuo_db_key       = "transfer_pull_task_if_atuo_db_key"
	Transfer_p2p_mgs_timeout                = 5 * time.Second
)

const (
	Transfer_pull_task_stautus_pending_confirmation = "pendingConfirmation" //任务待确认
	Transfer_pull_task_stautus_running              = "running"             //运行中
	Transfer_pull_task_stautus_stop                 = "stop"                //已停止
)

type TransferManger struct {
	PushTask         *PushTask
	PullTask         *PullTask
	Exec             chan *PullTask
	TaskMap          *sync.Map
	PushTaskIDMax    uint64
	PullTaskIDMax    uint64
	PullIsAuto       bool //是否自动
	tcpMsg           *connTcpMessage.ConnTcp
	levelDB          *db.LevelDB
	lockPushTaskList *sync.RWMutex
	lockPullTaskList *sync.RWMutex
}

var TransferTaskManger *TransferManger

type PushTask struct {
	PushTaskID     uint64 `json:"push_task_id"`
	PullTaskID     uint64 `json:"pull_task_id"`
	To             string `json:"to"` //接收者
	Path           string `json:"path"`
	Hash           []byte `json:"hash"`
	ExpirationTime int64  `json:"expiration_time"`
}

type TransferMsg struct {
	PushTaskID uint64    `json:"push_task_id"`
	PullTaskID uint64    `json:"pull_task_id"`
	FileInfo   *FileInfo `json:"file_info"`
}

type FileInfo struct {
	From  string           `json:"from"` //传输者
	To    string           `json:"to"`   //接收者
	Name  string           `json:"name"` //原始文件名
	Hash  []byte           `json:"hash"`
	Size  int64            `json:"size"`
	Path  string           `json:"path"`
	Index int64            `json:"index"`
	Data  []byte           `json:"data"`
	Speed map[string]int64 `json:"speed"` //传输速度统计
	Rate  int64            `json:"rate"`
}

func NewTransferManger(tcpMsg *connTcpMessage.ConnTcp, transfer_push, transfer_pull, transfer_new_pull uint64) *TransferManger {
	msg_id_p2p_transfer_push = transfer_push
	msg_id_p2p_transfer_pull = transfer_pull
	msg_id_p2p_transfer_new_pull = transfer_new_pull

	tcpMsg.Router.AddRouter(msg_id_p2p_transfer_push, RecvNewPushTask)

	tcpMsg.Router.AddRouter(msg_id_p2p_transfer_pull, FileSlicePush)

	tcpMsg.Router.AddRouter(msg_id_p2p_transfer_new_pull, RecvNewPullTask)

	TransferTaskManger = &TransferManger{
		tcpMsg:           tcpMsg,
		levelDB:          db.GetDB(),
		lockPullTaskList: new(sync.RWMutex),
		lockPushTaskList: new(sync.RWMutex),
		Exec:             make(chan *PullTask, 20),
		TaskMap:          new(sync.Map),
	}
	return TransferTaskManger
}

// 加载任务列表
func (this *TransferManger) Load() {
	go this.begin()

	pushTaskIDMax, _ := this.levelDB.Find([]byte(Transfer_push_task_id_max))
	if pushTaskIDMax != nil {
		this.PushTaskIDMax = utils.BytesToUint64(*pushTaskIDMax)
	}

	pullTaskIDMax, _ := this.levelDB.Find([]byte(Transfer_pull_task_id_max))
	if pullTaskIDMax != nil {
		this.PullTaskIDMax = utils.BytesToUint64(*pullTaskIDMax)
	}

	//加载拉取任务
	list, err := this.PullTask.PullTaskRecordList()
	if err != nil {
		return
	}
	for _, v := range list {
		if v.Status == Transfer_pull_task_stautus_running {
			v.CreatePullTask()
		} else {
			this.TaskMap.Store(v.PullTaskID, v)
		}
	}
}

func (this *TransferManger) begin() {
	var timer = time.NewTicker(Transfer_task_expiration_interval)
	defer timer.Stop()
	for {
		select {
		case <-timer.C: //定时清理过期推送任务
			go this.PushTask.clearExpirationPushTask()
		case s := <-this.Exec: //执行一个拉取任务
			go s.Task()
		}
	}
}

/*
TransferPushTaskSharingDirs
@Description: 共享目录列表
@receiver this
@return sharingDirs
@return err
*/
func (this *TransferManger) TransferPushTaskSharingDirs() (sharingDirs map[string]string, err error) {
	return this.PushTask.TransferPushTaskSharingDirs()
}

/*
TransferPushTaskSharingDirsAdd
@Description: 共享目录添加
@receiver this
@param dir 目录绝对路径
@return error
*/
func (this *TransferManger) TransferPushTaskSharingDirsAdd(dir string) error {
	return this.PushTask.TransferPushTaskSharingDirsAdd(dir)
}

/*
TransferPushTaskSharingDirsDel
@Description: 共享目录删除
@receiver this
@param dir 目录绝对路径
@return error
*/
func (this *TransferManger) TransferPushTaskSharingDirsDel(dir string) error {
	return this.PushTask.TransferPushTaskSharingDirsDel(dir)
}

/*
TransferPullAddrWhiteList
@Description: 授权白名单地址列表
@receiver this
@return whiteList
@return err
*/
func (this *TransferManger) TransferPullAddrWhiteList() (whiteList []string, err error) {
	return this.PushTask.TransferPullAddrWhiteList()
}

/*
TransferPullAddrWhiteListAdd
@Description: 授权白名单地址添加
@receiver this
@param addr
@return error
*/
func (this *TransferManger) TransferPullAddrWhiteListAdd(addr string) error {
	return this.PushTask.TransferPullAddrWhiteListAdd(addr)
}

/*
TransferPullAddrWhiteListDel
@Description: 授权白名单地址删除
@receiver this
@param addr
@return error
*/
func (this *TransferManger) TransferPullAddrWhiteListDel(addr string) error {
	return this.PushTask.TransferPullAddrWhiteListDel(addr)
}

/*
NewPushTask
@Description: 传输文件申请(发起一个推送任务)
@receiver this
@param path
@param to
@return error
*/
func (this *TransferManger) NewPushTask(path, to string) (*TransferMsg, error) {
	this.PushTask = &PushTask{
		To:             to,
		Path:           path,
		ExpirationTime: time.Now().Unix() + int64(Transfer_task_expiration_interval.Seconds()),
	}
	tm, err := this.PushTask.CreatePushTask()
	if err != nil {
		return nil, err
	}
	return tm, nil
}

/*
NewPullTask
@Description: 发起一个拉取文件任务
@receiver this
@param source 资源相对路径如:files/text.txt
@param path 相对路径如:files/text.txt
@param from 文件来源节点地址
@return error
*/
func (this *TransferManger) NewPullTask(source, path, from string) error {
	//向from发起验证白名和共享文件夹，成功则加入拉取任务列表，
	sendMsg := TransferMsg{
		PullTaskID: TransferTaskManger.newTaskID(Transfer_pull_task_id_max),
		FileInfo: &FileInfo{
			To:   TransferTaskManger.tcpMsg.GetSelfAddr(),
			From: from,
			Path: source,
		},
	}

	fd, err := json.Marshal(sendMsg)
	if err != nil {
		log.Log.Error(fmt.Sprintf("json.Marshal失败:%s", err.Error()))
		return err
	}
	message, ok, err := TransferTaskManger.tcpMsg.Send(nil, from, msg_id_p2p_transfer_new_pull, fd, "", Transfer_p2p_mgs_timeout)
	if !ok {
		log.Log.Error(fmt.Sprintf("发送P2p消息失败:%s", err.Error()))
		return err
	}

	if err != nil {
		log.Log.Error(fmt.Sprintf("对方节点创建推送任务失败:%s", err.Error()))
		return err
	}
	if message != nil && len(message) > 0 {
		m, err := ParseMsg(message)
		if err != nil {
			log.Log.Error(fmt.Sprintf("P2p消息解析失败:%s", err.Error()))
			return err
		}

		//临时文件
		if path == "" {
			m.FileInfo.Path = filepath.Join(recfilepath, m.FileInfo.Name)
		} else {
			m.FileInfo.Name = filepath.Base(path)
			m.FileInfo.Path = path
		}
		//添加传输任务
		this.PullTask = &PullTask{
			PushTaskID: m.PushTaskID,
			PullTaskID: m.PullTaskID,
			FileInfo:   m.FileInfo,
			exit:       make(chan struct{}),
			Status:     Transfer_pull_task_stautus_running,
		}
		err = this.PullTask.CreatePullTask()
		if err != nil {
			return err
		}
	} else {
		log.Log.Error(fmt.Sprintf("对方节点创建推送任务失败"))
		return errors.New("对方节点创建推送任务失败")
	}

	return nil
}

/*
PullTaskStart
@Description: 推送任务开始
@receiver this
@param taskId 推送任务id
@return ok
@return err
*/
func (this *TransferManger) PullTaskStart(taskId uint64, path string) error {
	task, err := this.PullTask.pullTaskRecordOne(taskId)
	if err != nil {
		return err
	}

	if task.Status == Transfer_pull_task_stautus_running {
		return errors.New("不能重复开启该任务")
	}

	if task.Status == Transfer_pull_task_stautus_stop && path != "" {
		return errors.New("该任务状态下不能更改存储目录")
	}

	//判断原来的零时目录
	if task.Status == Transfer_pull_task_stautus_pending_confirmation {
		//修改目录
		if path == "" {
			task.FileInfo.Path = filepath.Join(recfilepath, task.FileInfo.Name)
		} else {
			task.FileInfo.Name = filepath.Base(path)
			task.FileInfo.Path = path
		}
	}

	task.Status = Transfer_pull_task_stautus_running
	err = task.CreatePullTask()
	if err != nil {
		return err
	}
	return nil
}

/*
PullTaskStop
@Description: 推送任务停止
@receiver this
@param taskId 推送任务id
@return ok
@return err
*/
func (this *TransferManger) PullTaskStop(taskId uint64) error {
	//发送拉取消息
	v, ok := TransferTaskManger.TaskMap.Load(taskId)
	if !ok {
		return errors.New("未找到该任务")
	}
	task := v.(*PullTask)
	if task.Status == Transfer_pull_task_stautus_stop {
		return errors.New("不能重复停止该任务")
	}

	if task.exit == nil {
		return errors.New("不能停止该任务！")
	}

	task.exit <- struct{}{}
	return nil
}

/*
PullTaskDel
@Description: 推送任务删除
@receiver this
@param taskId 推送任务id
@return ok
@return err
*/
func (this *TransferManger) PullTaskDel(taskId uint64) error {
	task, err := this.PullTask.pullTaskRecordOne(taskId)
	if err != nil {
		return err
	}
	if task.Status == Transfer_pull_task_stautus_running {
		task.clear <- struct{}{}
	} else {
		err := task.pullTaskDel()
		return err
	}
	return nil
}

/*
PullTaskList
@Description: 推送任务列表
@receiver this
@return []*PullTask
*/
func (this *TransferManger) PullTaskList() []*PullTask {
	list, _ := this.PullTask.PullTaskRecordList()
	return list
}

/*
PullTaskIsAutoSet
@Description: 设置是否自动拉取
@receiver this
@param auto  ture开启自动拉取
@return error
*/
func (this *TransferManger) PullTaskIsAutoSet(auto bool) error {
	b := []byte(strconv.FormatBool(auto))
	return TransferTaskManger.levelDB.Save([]byte(Transfer_pull_task_if_atuo_db_key), &b)
}

/*
PullTaskIsAutoGet
@Description: 获取是否自动拉取状态
@receiver this
@return bool ture开启自动拉取
@return error
*/
func (this *TransferManger) PullTaskIsAutoGet() (bool, error) {
	return this.PullTask.PullTaskIsAutoGet()
}

// 创建一个push任务
func (t *PushTask) CreatePushTask() (*TransferMsg, error) {
	if !filepath.IsAbs(t.Path) {
		return nil, errors.New("文件路径必须是绝对路径！")
	}
	fi, err := os.Stat(t.Path)
	if err != nil {
		log.Log.Error(fmt.Sprintf("文件Stat失败:%s", err.Error()))
		return nil, err
	}
	hasB, err := utils.FileSHA3_256(t.Path)
	if err != nil {
		log.Log.Error(fmt.Sprintf("文件hash失败:%s", err.Error()))
		return nil, err
	}

	sendMsg := TransferMsg{
		PullTaskID: t.PullTaskID,
		PushTaskID: TransferTaskManger.newTaskID(Transfer_push_task_id_max),
		FileInfo: &FileInfo{
			To:   t.To,
			From: TransferTaskManger.tcpMsg.GetSelfAddr(),
			Name: fi.Name(),
			Size: fi.Size(),
			Hash: hasB,
		},
	}

	if sendMsg.FileInfo.Size < 1 {
		log.Log.Error(fmt.Sprintf("文件大小为空不能传输"))
		return nil, err
	}

	if t.PullTaskID == 0 {
		fd, err := json.Marshal(sendMsg)
		if err != nil {
			log.Log.Error(fmt.Sprintf("json.Marshal失败:%s", err.Error()))
			return nil, err
		}
		message, ok, err := TransferTaskManger.tcpMsg.Send(nil, sendMsg.FileInfo.To, msg_id_p2p_transfer_push, fd, "", Transfer_p2p_mgs_timeout)
		if !ok {
			log.Log.Error(fmt.Sprintf("发送P2p消息失败:%s", err.Error()))
			return nil, err
		}
		if err != nil {
			log.Log.Error(fmt.Sprintf("对方节点创建拉取任务失败:%s", err.Error()))
			return nil, err
		}
		if message != nil && len(message) > 0 {
			recv, err := ParseMsg(message)
			if err != nil {
				log.Log.Error(fmt.Sprintf("P2p消息解析失败:%s", err.Error()))
				return nil, err
			}
			if recv.PullTaskID < 1 {
				log.Log.Error("接收方任务id为空！！")
				return nil, err
			}
			t.PullTaskID = recv.PullTaskID
		} else {
			log.Log.Error("对方节点创建拉取任务失败")
			return nil, errors.New("对方节点创建拉取任务失败")
		}
	}

	t.PushTaskID = sendMsg.PushTaskID
	t.Hash = hasB
	err = t.pushTaskRecordSave()
	if err != nil {
		log.Log.Error(fmt.Sprintf(err.Error()))
		return nil, err
	}
	return &sendMsg, nil

}

func (t *PushTask) pushTaskRecordSave() error {
	taskList, _ := t.pushTaskRecordList()
	if taskList == nil {
		taskList = make(map[uint64]*PushTask)
	}

	TransferTaskManger.lockPushTaskList.Lock()
	defer TransferTaskManger.lockPushTaskList.Unlock()

	taskList[t.PushTaskID] = t
	fd, err := json.Marshal(taskList)
	if err != nil {
		return err
	}
	return TransferTaskManger.levelDB.Save([]byte(Transferlog_push_task_db_key), &fd)
}
func (t *PushTask) pushTaskRecordDel() error {
	taskList, _ := t.pushTaskRecordList()
	if taskList == nil {
		return nil
	}

	TransferTaskManger.lockPushTaskList.Lock()
	defer TransferTaskManger.lockPushTaskList.Unlock()

	delete(taskList, t.PushTaskID)
	fd, err := json.Marshal(taskList)
	if err != nil {
		return err
	}
	return TransferTaskManger.levelDB.Save([]byte(Transferlog_push_task_db_key), &fd)
}
func (t *PushTask) pushTaskRecordList() (taskList map[uint64]*PushTask, err error) {
	TransferTaskManger.lockPushTaskList.RLock()
	defer TransferTaskManger.lockPushTaskList.RUnlock()
	b, err := TransferTaskManger.levelDB.Find([]byte(Transferlog_push_task_db_key))
	if err != nil {
		//log.Log.Info(fmt.Sprintf("没有获取到db中的任务列表%s", err.Error()))
		return nil, err
	}

	if b == nil {
		log.Log.Error("没有获取到db中的任务列表")
		return nil, errors.New("没有获取到任务列表")
	}

	decoder := json.NewDecoder(bytes.NewBuffer(*b))
	decoder.UseNumber()
	err = decoder.Decode(&taskList)
	if err != nil {
		log.Log.Error(fmt.Sprintf("db中的任务列表解析失败%s", err.Error()))
		return
	}
	return
}

func (t *PushTask) checkPushTask(pushTaskId, pullTaskId uint64, sendId string) (task *PushTask, err error) {
	taskList, err := t.pushTaskRecordList()
	if err != nil {
		return nil, err
	}
	task, ok := taskList[pushTaskId]
	if !ok {
		return nil, errors.New("没有该任务，无权限！！")
	}
	if task.To != sendId {
		return nil, errors.New("节点无权限！！")
	}
	if task.PullTaskID != pullTaskId {
		return nil, errors.New("节点无任务权限！！")
	}
	if task.ExpirationTime > 0 && task.ExpirationTime <= time.Now().Unix() {
		return nil, errors.New("任务已过期！！")
	}

	return
}

func (t *PushTask) TransferPushTaskSharingDirs() (sharingDirs map[string]string, err error) {
	b, err := TransferTaskManger.levelDB.Find([]byte(Transfer_push_task_sharing_dirs))
	if err != nil {
		log.Log.Info(fmt.Sprintf("没有获取到db中的共享目录%s", err.Error()))
		return
	}

	if b == nil {
		log.Log.Error("没有获取到db中的共享目录")
		err = errors.New("没有获取到db中的共享目录")
		return
	}

	decoder := json.NewDecoder(bytes.NewBuffer(*b))
	decoder.UseNumber()
	err = decoder.Decode(&sharingDirs)
	if err != nil {
		log.Log.Error(fmt.Sprintf("db中的共享目录列表解析失败%s", err.Error()))
		return
	}
	return
}

func (t *PushTask) TransferPushTaskSharingDirsAdd(dir string) error {
	fi, err := os.Stat(dir)
	if err != nil {
		return err
	}

	if !fi.IsDir() {
		return errors.New("不是一个有效目录")
	}

	key := filepath.Base(strings.TrimSuffix(filepath.ToSlash(dir), "/")) //获取最后一个目录名

	if key == "" {
		return errors.New("获取最后一个目录名失败")
	}

	list, _ := t.TransferPushTaskSharingDirs()
	if list == nil {
		list = make(map[string]string)
	} else {
		_, ok := list[key]
		if ok {
			return errors.New("目录key值重复，请更换最后一级目录名")
		}
	}

	list[key] = dir
	fd, err := json.Marshal(list)
	if err != nil {
		return err
	}
	return TransferTaskManger.levelDB.Save([]byte(Transfer_push_task_sharing_dirs), &fd)
}

func (t *PushTask) TransferPushTaskSharingDirsDel(dir string) error {

	fi, err := os.Stat(dir)
	if err != nil {
		return err
	}

	if !fi.IsDir() {
		return errors.New("不是一个有效目录")
	}

	key := filepath.Base(strings.TrimSuffix(filepath.ToSlash(dir), "/")) //获取最后一个目录名

	if key == "" {
		return errors.New("获取最后一个目录名失败")
	}

	list, err := t.TransferPushTaskSharingDirs()
	if err != nil {
		return err
	}

	delete(list, key)

	fd, err := json.Marshal(list)
	if err != nil {
		return err
	}
	return TransferTaskManger.levelDB.Save([]byte(Transfer_push_task_sharing_dirs), &fd)
}

func (t *PushTask) TransferPullAddrWhiteList() (whiteList []string, err error) {
	b, err := TransferTaskManger.levelDB.Find([]byte(Transfer_pull_addr_white_list))
	if err != nil {
		//log.Log.Error(fmt.Sprintf("没有获取到db中拉取地址的白名单%s", err.Error())
		return
	}

	if b == nil {
		//log.Log.Error(fmt.Sprintf("没有获取到db中拉取地址的白名单")
		err = errors.New("没有获取到db中拉取地址的白名单")
		return
	}

	decoder := json.NewDecoder(bytes.NewBuffer(*b))
	decoder.UseNumber()
	err = decoder.Decode(&whiteList)
	if err != nil {
		log.Log.Error(fmt.Sprintf("db中拉取地址的白名单列表解析失败%s", err.Error()))
		return
	}
	return
}

func (t *PushTask) TransferPullAddrWhiteListAdd(addr string) error {
	list, _ := t.TransferPullAddrWhiteList()
	for k, _ := range list {
		if list[k] == addr {
			return nil
		}
	}
	list = append(list, addr)
	fd, err := json.Marshal(list)
	if err != nil {
		return err
	}
	return TransferTaskManger.levelDB.Save([]byte(Transfer_pull_addr_white_list), &fd)
}

func (t *PushTask) TransferPullAddrWhiteListDel(addr string) error {
	list, err := t.TransferPullAddrWhiteList()
	if err != nil {
		return err
	}
	for k, _ := range list {
		if list[k] == addr {
			list = append(list[:k], list[k+1:]...)
			break
		}
	}
	fd, err := json.Marshal(list)
	if err != nil {
		return err
	}
	return TransferTaskManger.levelDB.Save([]byte(Transfer_pull_addr_white_list), &fd)
}

func (t *PushTask) clearExpirationPushTask() {
	taskList, _ := t.pushTaskRecordList()
	if len(taskList) == 0 {
		return
	}

	TransferTaskManger.lockPushTaskList.Lock()
	defer TransferTaskManger.lockPushTaskList.Unlock()
	newT := time.Now().Unix()
	var update bool
	for _, v := range taskList {
		if v.ExpirationTime > 0 && v.ExpirationTime <= newT {
			delete(taskList, v.PushTaskID)
			update = true
		}
	}

	if update {
		fd, err := json.Marshal(taskList)
		if err != nil {
			log.Log.Info(fmt.Sprintf("没有获取到db中的任务列表%s", err.Error()))
			return
		}
		TransferTaskManger.levelDB.Save([]byte(Transferlog_push_task_db_key), &fd)
	}

	return
}

// 采集速度参数
func (f *FileInfo) SetSpeed(stime int64, size int) error {
	if _, ok := f.Speed["time"]; !ok {
		f.Speed["time"] = stime
		f.Speed["size"] = int64(size)
	}
	if time.Now().Unix()-f.Speed["time"] > Second {
		f.Speed["time"] = stime
		f.Speed["size"] = 0
	} else {
		f.Speed["size"] += int64(size)
	}
	return nil
}

// 获取速度
func (f *FileInfo) GetSpeed() int64 {
	t := time.Now().Unix() - f.Speed["time"]
	if t < 1 {
		t = 1
	}
	return f.Speed["size"] / t / 1024
}

func (f *FileInfo) CheckFileHash() (err error) {
	hasB, err := utils.FileSHA3_256(f.Path)
	if err != nil {
		log.Log.Error(fmt.Sprintf("文件hash失败:%s", err.Error()))
		return err
	}
	if !bytes.Equal(hasB, f.Hash) {
		log.Log.Error("文件上传错误！不完整或已损坏")
		return errors.New("文件上传错误！不完整或已损坏")
	}
	return err
}

// 解析消息
func ParseMsg(d []byte) (*TransferMsg, error) {
	msg := &TransferMsg{}
	// err := json.Unmarshal(d, msg)
	decoder := json.NewDecoder(bytes.NewBuffer(d))
	decoder.UseNumber()
	err := decoder.Decode(msg)
	if err != nil {
		fmt.Println(err)
	}
	return msg, err
}

// 对ip和端口进行处理让
func addrToNumStr(s string) string {
	h := fnv.New32a()
	h.Write([]byte(s))
	return fmt.Sprintf("%d", h.Sum32())
}
