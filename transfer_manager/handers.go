package transfer_manager

import (
	"bytes"
	"encoding/json"
	"errors"
	"file_transfer_manager/connTcpMessage"
	"file_transfer_manager/pkg/utils"
	log "file_transfer_manager/pkg/zap"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	//"sync"
	"context"
	"sync/atomic"
	"time"
)

var msg_id_p2p_transfer_push uint64 = 1001 //请求传输文件

var msg_id_p2p_transfer_pull uint64 = 1003 //请求拉取文件流

var msg_id_p2p_transfer_new_pull uint64 = 1005 //创建拉取任务

// 接收传输文件申请并创建pull任务

func RecvNewPushTask(ctx context.Context, message *connTcpMessage.Message) ([]byte, error) {
	content := message.Data
	m, err := ParseMsg(content)
	if err == nil {

		pullTaskID := TransferTaskManger.newTaskID(Transfer_pull_task_id_max)
		//回复任务id
		m.PullTaskID = pullTaskID
		recv, err := json.Marshal(m)
		if err != nil {
			fmt.Println(err)
		}

		go func() {
			//添加传输任务
			task := &PullTask{
				PushTaskID: m.PushTaskID,
				PullTaskID: pullTaskID,
				FileInfo:   m.FileInfo,
			}
			//路径默认为
			task.FileInfo.Path = filepath.Join(recfilepath, m.FileInfo.Name)
			task.Status = Transfer_pull_task_stautus_running
			//判断是否自动拉取
			autoPull, _ := task.PullTaskIsAutoGet()
			if !autoPull {
				task.Status = Transfer_pull_task_stautus_pending_confirmation
			}
			// 1秒之后执行，防止消息还未到Push端时push任务还未完全建立好这时间去拉取就会异常
			time.Sleep(1 * time.Second)
			task.CreatePullTask()
			return
		}()

		return recv, nil
	}
	log.Log.Error(fmt.Sprintf("p2p消息解析败:%s", err.Error()))
	return nil, err
}

// 接收一个Pull任务申请，并创建对应的push任务
func RecvNewPullTask(ctx context.Context, message *connTcpMessage.Message) ([]byte, error) {
	content := message.Data
	m, err := ParseMsg(content)
	if err == nil {
		//验证白名单
		var task *PushTask
		whiteList, _ := task.TransferPullAddrWhiteList()
		var have bool
		for k, _ := range whiteList {
			if whiteList[k] == m.FileInfo.To {
				have = true
				break
			}
		}
		if !have {
			log.Log.Info("验证白名单失败,节点无权限")
			return nil, errors.New("验证白名单失败,节点无权限")
		}

		//验证地址
		sharingDirs, _ := task.TransferPushTaskSharingDirs()
		strs := strings.Split(strings.TrimLeft(strings.TrimLeft(filepath.ToSlash(m.FileInfo.Path), "./"), "/"), "/")
		if len(strs) <= 1 {
			log.Log.Info("解析共享目录地址key失败，不是有效路径！")
			return nil, errors.New("解析共享目录地址key失败，不是有效路径！")
		}
		dir, ok := sharingDirs[strs[0]]
		if !ok {
			log.Log.Info("解析共享目录地址key失败，无权限")
			return nil, errors.New("解析共享目录地址key失败，无权限！")
		}
		//创建推送任务
		task = &PushTask{
			PullTaskID: m.PullTaskID,
			To:         m.FileInfo.To,
			Path:       filepath.Join(dir, filepath.Join(strs[1:]...)),
		}
		sendMsg, err := task.CreatePushTask()
		if err != nil {
			log.Log.Info(fmt.Sprintf("创建推送任务失败s%", err.Error()))
			return nil, err
		}

		fd, err := json.Marshal(sendMsg)
		if err != nil {
			log.Log.Error(fmt.Sprintf("json.Marshal失败:%s", err.Error()))
			return nil, err
		}
		return fd, nil
	}
	log.Log.Error(fmt.Sprintf("解析msg失败s%", err.Error()))
	return nil, errors.New(fmt.Sprintf("解析msg失败s%", err.Error()))
}

func (t *PullTask) PullTaskIsAutoGet() (bool, error) {
	b, err := TransferTaskManger.levelDB.Find([]byte(Transfer_pull_task_if_atuo_db_key))
	if err != nil {
		return false, err
	}

	auto, err := strconv.ParseBool(string(*b))
	if err != nil {
		return false, err
	}
	return auto, nil
}

func (this *TransferManger) newTaskID(idtype string) uint64 {
	if idtype == Transfer_push_task_id_max {
		b := utils.Uint64ToBytes(atomic.AddUint64(&this.PushTaskIDMax, 1))
		this.levelDB.Save([]byte(Transfer_push_task_id_max), &b)
		return this.PushTaskIDMax
	}
	if idtype == Transfer_pull_task_id_max {
		b := utils.Uint64ToBytes(atomic.AddUint64(&this.PullTaskIDMax, 1))
		this.levelDB.Save([]byte(Transfer_pull_task_id_max), &b)
		return this.PullTaskIDMax
	}
	return 0
}

func (t *PullTask) PullTaskRecordList() (taskList []*PullTask, err error) {
	TransferTaskManger.lockPullTaskList.RLock()
	defer TransferTaskManger.lockPullTaskList.RUnlock()
	b, err := TransferTaskManger.levelDB.Find([]byte(Transferlog_pull_task_db_key))
	if err != nil {
		//log.Log.Info(fmt.Sprintf("没有获取到db中的任务列表%s", err.Error()))
		return nil, err
	}
	if b == nil {
		log.Log.Error("没有获取到db中的任务列表")
		return taskList, errors.New("没有获取到任务列表")
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

// 拉取文件任务
type PullTask struct {
	PushTaskID uint64    `json:"push_task_id"`
	PullTaskID uint64    `json:"pull_task_id"`
	FileInfo   *FileInfo `json:"file_info"`
	Status     string    `json:"status"` //ture 为传输中 false为暂停中
	Fault      string    `json:"fault"`  //传输文件过程中的异常
	exit       chan struct{}
	clear      chan struct{}
}

func (t *PullTask) CreatePullTask() error {
	log.Log.Info(fmt.Sprintf("任务id:%d", t.PullTaskID))
	//初始化
	t.exit = make(chan struct{})
	t.clear = make(chan struct{})
	t.FileInfo.Speed = make(map[string]int64, 0)
	t.Fault = ""
	//保存任务
	err := t.pullTaskRecordSave()
	if err != nil {
		log.Log.Error(fmt.Sprintf("保存错误:%s", err.Error()))
		return err
	}

	if t.Status == Transfer_pull_task_stautus_running {
		dir := filepath.Dir(t.FileInfo.Path)
		utils.CheckCreateDir(dir)
		tmpPath := filepath.Join(dir, t.FileInfo.Name+"_"+addrToNumStr(t.FileInfo.From)+"_"+strconv.FormatUint(t.PullTaskID, 10)+"_tmp")
		fi, err := os.OpenFile(tmpPath, os.O_RDWR|os.O_CREATE, os.ModePerm)
		if err != nil {
			log.Log.Error(fmt.Sprintf("临时文件新建失败:%s", err.Error()))
			return err
		}
		defer fi.Close()

		err = fi.Truncate(t.FileInfo.Size)
		if err != nil {
			log.Log.Error(fmt.Sprintf("空文件新建失败:%s", err.Error()))
			return err
		}

		TransferTaskManger.Exec <- t
	}
	return nil
}

func (t *PullTask) Task() {
	var errnum int
	conn, err := net.DialTimeout("tcp", t.FileInfo.From, Transfer_p2p_mgs_timeout)
	if err != nil {
		log.Log.Error(fmt.Sprintf("Dial错误:%s", err.Error()))
		return
	}
	defer conn.Close()

	for {
		select {
		case <-t.exit:
			t.Status = Transfer_pull_task_stautus_stop
			log.Log.Info(fmt.Sprintf("退出任务id:%d", t.PullTaskID))
			err = t.pullTaskRecordSave()
			if err != nil {
				log.Log.Error(fmt.Sprintf("保存错误:%s", err.Error()))
			}
			return
		case <-t.clear:
			log.Log.Info(fmt.Sprintf("删除任务id:%d", t.PullTaskID))
			err = t.pullTaskDel()
			if err != nil {
				log.Log.Error(fmt.Sprintf("删除任务错误:%s", err.Error()))
			}
			return
		default:
			okf, err := t.fileSlicePull(conn)
			if okf == true { //已传输完，则退出
				t.Status = Transfer_pull_task_stautus_stop
				err = t.pullTaskRecordDel()
				if err != nil {
					log.Log.Error(fmt.Sprintf("已传输完，清理错误:%s", err.Error()))
				}
				return
			}
			if err != nil {
				//开始重传
				errnum++
				if errnum <= ErrNum {
					fmt.Println("resend slice...")
					continue
				}
				t.Status = Transfer_pull_task_stautus_stop
				t.Fault = err.Error()
				err = t.pullTaskRecordSave()
				if err != nil {
					log.Log.Error(fmt.Sprintf("保存错误:%s", err.Error()))
				}
				return
			}
		}
	}

}

//分段传输，续传
/**
@return okf 是否传送完 err 错误
*/
func (t *PullTask) fileSlicePull(conn net.Conn) (okf bool, errs error) {
	//已经传完
	if t.FileInfo.Index >= t.FileInfo.Size && t.FileInfo.Size > 0 {
		return true, nil
	}

	fd, err := json.Marshal(t)
	if err != nil {
		fmt.Println(err)
	}
	message, ok, err := TransferTaskManger.tcpMsg.Send(conn, "", msg_id_p2p_transfer_pull, fd, "", Transfer_p2p_mgs_timeout)
	if !ok {
		log.Log.Error(fmt.Sprintf("发送P2p消息失败:%s", err.Error()))
		return false, err
	}
	if err != nil {
		log.Log.Error(fmt.Sprintf("对方节点推送失败:%s", err.Error()))
		return false, err
	}
	if message != nil && len(message) > 0 {
		//发送成功，对方已经接收到消息
		m, err := ParseMsg(message)
		if err != nil {
			log.Log.Error(fmt.Sprintf("P2p消息解析失败:%s", err.Error()))
			return false, err
		}

		tmpPath := filepath.Join(filepath.Dir(t.FileInfo.Path), t.FileInfo.Name+"_"+addrToNumStr(t.FileInfo.From)+"_"+strconv.FormatUint(t.PullTaskID, 10)+"_tmp")
		fi, err := os.OpenFile(tmpPath, os.O_RDWR, os.ModePerm)
		if err != nil {
			log.Log.Error(fmt.Sprintf("临时文件打开失败:%s", err.Error()))
			return false, err
		}
		fi.Seek(t.FileInfo.Index, 0)
		fi.Write(m.FileInfo.Data)
		defer fi.Close()

		t.FileInfo.Index = m.FileInfo.Index

		log.Log.Info(fmt.Sprintf("接收的start：%d", t.FileInfo.Index))
		t.FileInfo.Rate = int64(float64(m.FileInfo.Index) / float64(m.FileInfo.Size) * float64(100))
		log.Log.Info(fmt.Sprintf("接收的百分比：%d%%", t.FileInfo.Rate))
		t.FileInfo.SetSpeed(time.Now().Unix(), len(message))
		speed := t.FileInfo.GetSpeed()
		log.Log.Info(fmt.Sprintf("接收的速率：%d KB/s", speed))
		//传输完成，则更新状态
		if t.FileInfo.Rate >= 100 {
			//传输完成，则重命名文件名
			num := 0
		Rename:
			//如果文件存在，则重命名为新的文件
			if ok, _ := utils.PathExists(t.FileInfo.Path); ok {
				num++
				filenamebase := filepath.Base(t.FileInfo.Name)
				fileext := filepath.Ext(t.FileInfo.Name)
				filename := strings.TrimSuffix(filenamebase, fileext)
				newname := filename + "_" + strconv.Itoa(num) + fileext
				t.FileInfo.Path = filepath.Join(filepath.Dir(t.FileInfo.Path), newname)
				if ok1, _ := utils.PathExists(t.FileInfo.Path); ok1 {
					goto Rename
				}
				t.FileInfo.Name = newname
			}

			fi.Close()
			os.Rename(tmpPath, t.FileInfo.Path)
			log.Log.Info(fmt.Sprintf("PullTaskID %d 文件完成：%d %d%%", t.PullTaskID, t.FileInfo.Index, t.FileInfo.Rate))
			//检查文件完整性
			err = t.FileInfo.CheckFileHash()
			okf = true // 发送完成
			return
		} else {
			t.pullTaskRecordSave()
		}
	} else {
		log.Log.Error("文件拉取失败")
		return false, errors.New("文件拉取失败")
	}

	return
}

// 分片推送
func FileSlicePush(ctx context.Context, message *connTcpMessage.Message) ([]byte, error) {
	content := message.Data
	m, err := ParseMsg(content)
	if err == nil {
		//验证taskID
		var task *PushTask
		task, err := task.checkPushTask(m.PushTaskID, m.PullTaskID, message.Sender)
		if err != nil {
			log.Log.Error(fmt.Sprintf("验证task失败：%s", err.Error()))
			return nil, err
		}

		index := m.FileInfo.Index //当前已传偏移量
		fi, err := os.Open(task.Path)
		if err != nil {
			log.Log.Error(fmt.Sprintf("文件打开失败：%s", err.Error()))
			return nil, err
		}
		defer fi.Close()
		stat, err := fi.Stat()
		if err != nil {
			log.Log.Error(fmt.Sprintf("文件打开失败：%s", err.Error()))
			return nil, err
		}
		size := stat.Size()
		start := index
		length := Lenth
		//如果偏移量小于文件大小，并且剩余大小小于长度，则长度为剩余大小(即最后一段)
		if start < size && size-index < Lenth {
			length = size - index
		}
		buf := make([]byte, length)
		_, err = fi.ReadAt(buf, start)
		if err != nil {
			log.Log.Error(fmt.Sprintf("读取文件流失败：%s", err.Error()))
			return nil, err
		}
		//下一次start位置
		nextstart := start + Lenth
		if nextstart > size {
			nextstart = size
		}
		m.FileInfo.Size = size
		m.FileInfo.Index = nextstart
		m.FileInfo.Data = buf

		fd, err := json.Marshal(m)
		if err != nil {
			log.Log.Error(fmt.Sprintf("Marshal err:%s", err.Error()))
			return nil, err
		}
		//更新推送日志
		rate := int64(float64(nextstart) / float64(size) * float64(100))
		log.Log.Info(fmt.Sprintf("推送的百分比：%d%%", rate))
		if nextstart >= size {
			log.Log.Info("文件推送完成")
			task.pushTaskRecordDel()
		}

		return fd, nil
	} else {
		log.Log.Error(fmt.Sprintf("P2p消息解析失败：%s", err.Error()))
		return nil, err
	}
}

func (t *PullTask) pullTaskDel() error {
	err := t.pullTaskRecordDel()
	if err != nil {
		return err
	}
	//删除临时文件
	tmpPath := filepath.Join(filepath.Dir(t.FileInfo.Path), t.FileInfo.Name+"_"+addrToNumStr(t.FileInfo.From)+"_"+strconv.FormatUint(t.PullTaskID, 10)+"_tmp")
	ok, err := utils.PathExists(tmpPath)
	if err != nil {
		log.Log.Error(fmt.Sprintf("删除临时文件失败", err.Error()))
		return err
	}
	if ok {
		err = os.Remove(tmpPath)
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *PullTask) pullTaskRecordOne(pullTaskID uint64) (task *PullTask, err error) {
	//先在内存中拿
	if v, ok := TransferTaskManger.TaskMap.Load(pullTaskID); ok {
		task = v.(*PullTask)
		return
	}
	//拿不到就去db里拿
	taskList, err := t.PullTaskRecordList()
	if err != nil {
		return
	}

	for _, v := range taskList {
		if v.PullTaskID == pullTaskID {
			TransferTaskManger.TaskMap.Store(pullTaskID, v)
			return v, nil
		}
	}

	return nil, errors.New("任务不存在")
}

func (t *PullTask) pullTaskRecordDel() error {
	taskList, err := t.PullTaskRecordList()
	if err != nil {
		return err
	}

	TransferTaskManger.lockPullTaskList.Lock()
	defer TransferTaskManger.lockPullTaskList.Unlock()
	TransferTaskManger.TaskMap.Delete(t.PullTaskID)

	for k, v := range taskList {
		if v.PullTaskID == t.PullTaskID {
			taskList = append(taskList[:k], taskList[k+1:]...)
			break
		}
	}

	fd, err := json.Marshal(taskList)
	if err != nil {
		return err
	}
	return TransferTaskManger.levelDB.Save([]byte(Transferlog_pull_task_db_key), &fd)
}

func (t *PullTask) pullTaskRecordSave() error {
	taskList, _ := t.PullTaskRecordList()

	TransferTaskManger.lockPullTaskList.Lock()
	defer TransferTaskManger.lockPullTaskList.Unlock()
	TransferTaskManger.TaskMap.Store(t.PullTaskID, t)

	have := false
	for k, v := range taskList {
		if v.PullTaskID == t.PullTaskID {
			taskList[k] = t
			have = true
			break
		}
	}
	if !have {
		taskList = append(taskList, t)
	}

	fd, err := json.Marshal(taskList)
	if err != nil {
		return err
	}
	return TransferTaskManger.levelDB.Save([]byte(Transferlog_pull_task_db_key), &fd)
}
