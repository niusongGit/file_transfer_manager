package main

import (
	"context"
	"file_transfer_manager/config"
	"file_transfer_manager/connTcpMessage"
	"file_transfer_manager/pkg/db"
	log "file_transfer_manager/pkg/zap"
	"file_transfer_manager/transfer_manager"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
)

func main() {
	config.Step()
	log.Step(config.LogPath)
	defer log.Sync()

	tcpMsg := connTcpMessage.NewConnTcp(config.TcpIP, config.TcpPort)
	go tcpMsg.StartUP()
	RegisterMsgManger(tcpMsg.Router)

	db.InitDB("cache_data")

	transferManger := transfer_manager.NewTransferManger(tcpMsg, 1, 2, 3)
	transferManger.Load()

	// 创建一个 Gin 实例
	r := gin.Default()
	httpHander(r, transferManger)
	// 启动服务
	r.Run(fmt.Sprintf(":%d", config.WebPort))
}

func httpHander(r *gin.Engine, transferManger *transfer_manager.TransferManger) *gin.Engine {

	group := r.Group("/v1")

	///////////////////////////////// push端 //////////////////////////////////////////////

	//发送文件给一个节点
	pushG := group.Group("/push")
	pushG.POST("/task_new", func(c *gin.Context) {
		req := make(map[string]interface{})
		c.BindJSON(&req)

		path, ok := req["path"]
		if !ok {
			c.JSON(http.StatusOK, gin.H{"code": 999, "data": nil, "msg": "path 不能为空"})
			return
		}
		to, ok := req["to_addr"]
		if !ok {
			c.JSON(http.StatusOK, gin.H{"code": 999, "data": nil, "msg": "to_addr 接受地址不能为空"})
			return
		}

		res, err := transferManger.NewPushTask(path.(string), to.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 999, "data": nil, "msg": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"code": 200, "data": res, "msg": "success"})
		return
	})

	//共享目录列表
	pushG.POST("/sharing_dirs", func(c *gin.Context) {

		res, err := transferManger.TransferPushTaskSharingDirs()
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 999, "data": nil, "msg": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"code": 200, "data": res, "msg": "success"})
		return
	})
	//共享目录添加
	pushG.POST("/sharing_dirs_add", func(c *gin.Context) {
		req := make(map[string]interface{})
		c.BindJSON(&req)

		dir, ok := req["dir"]
		if !ok {
			c.JSON(http.StatusOK, gin.H{"code": 999, "data": nil, "msg": "dir 不能为空"})
			return
		}
		err := transferManger.TransferPushTaskSharingDirsAdd(dir.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 999, "data": nil, "msg": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"code": 200, "data": nil, "msg": "success"})
		return
	})
	//共享目录删除
	pushG.POST("/sharing_dirs_del", func(c *gin.Context) {
		req := make(map[string]interface{})
		c.BindJSON(&req)

		dir, ok := req["dir"]
		if !ok {
			c.JSON(http.StatusOK, gin.H{"code": 999, "data": nil, "msg": "dir 不能为空"})
			return
		}
		err := transferManger.TransferPushTaskSharingDirsDel(dir.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 999, "data": nil, "msg": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"code": 200, "data": nil, "msg": "success"})
		return
	})
	//授权白名单地址列表
	pushG.POST("/pull_addr_whitelist", func(c *gin.Context) {

		res, err := transferManger.TransferPullAddrWhiteList()
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 999, "data": nil, "msg": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"code": 200, "data": res, "msg": "success"})
		return
	})
	//授权白名单地址添加
	pushG.POST("/pull_addr_whitelist_add", func(c *gin.Context) {
		req := make(map[string]interface{})
		c.BindJSON(&req)

		addr, ok := req["addr"]
		if !ok {
			c.JSON(http.StatusOK, gin.H{"code": 999, "data": nil, "msg": "addr 不能为空"})
			return
		}
		err := transferManger.TransferPullAddrWhiteListAdd(addr.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 999, "data": nil, "msg": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"code": 200, "data": nil, "msg": "success"})
		return
	})
	//授权白名单地址删除
	pushG.POST("/pull_addr_whitelist_del", func(c *gin.Context) {
		req := make(map[string]interface{})
		c.BindJSON(&req)

		addr, ok := req["addr"]
		if !ok {
			c.JSON(http.StatusOK, gin.H{"code": 999, "data": nil, "msg": "addr 不能为空"})
			return
		}
		err := transferManger.TransferPullAddrWhiteListDel(addr.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 999, "data": nil, "msg": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"code": 200, "data": nil, "msg": "success"})
		return
	})

	///////////////////////////////// pull端 //////////////////////////////////////////////

	pullG := group.Group("/pull")
	//向一个节点发起拉取任务
	pullG.POST("/task_new", func(c *gin.Context) {
		req := make(map[string]interface{})
		c.BindJSON(&req)

		source, ok := req["source"]
		if !ok {
			c.JSON(http.StatusOK, gin.H{"code": 999, "data": nil, "msg": "source 不能为空"})
			return
		}
		path, ok := req["path"]
		if !ok {
			c.JSON(http.StatusOK, gin.H{"code": 999, "data": nil, "msg": "path 不能为空"})
			return
		}
		from, ok := req["from_addr"]
		if !ok {
			c.JSON(http.StatusOK, gin.H{"code": 999, "data": nil, "msg": "from_addr 来源地址不能为空"})
			return
		}

		err := transferManger.NewPullTask(source.(string), path.(string), from.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 999, "data": nil, "msg": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"code": 200, "data": nil, "msg": "success"})
		return
	})
	//设置是否自动拉取
	pullG.POST("/task_is_auto_set", func(c *gin.Context) {
		req := make(map[string]interface{})
		c.BindJSON(&req)

		auto, ok := req["auto"]
		if !ok {
			c.JSON(http.StatusOK, gin.H{"code": 999, "data": nil, "msg": "auto 不能为空"})
			return
		}
		err := transferManger.PullTaskIsAutoSet(auto.(bool))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 999, "data": nil, "msg": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"code": 200, "data": nil, "msg": "success"})
		return
	})
	//获取是否自动拉取状态
	pullG.POST("/task_is_auto_get", func(c *gin.Context) {
		auto, err := transferManger.PullTaskIsAutoGet()
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 999, "data": nil, "msg": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"code": 200, "data": auto, "msg": "success"})
		return
	})
	//获取拉取文件任务列表
	pullG.POST("/task_list", func(c *gin.Context) {
		list := transferManger.PullTaskList()
		c.JSON(http.StatusOK, gin.H{"code": 200, "data": list, "msg": "success"})
		return
	})
	//拉取文件任务停止
	pullG.POST("/task_stop", func(c *gin.Context) {
		req := make(map[string]interface{})
		c.BindJSON(&req)

		id, ok := req["task_id"]
		if !ok {
			c.JSON(http.StatusOK, gin.H{"code": 999, "data": nil, "msg": "task_id 不能为空"})
			return
		}
		err := transferManger.PullTaskStop(uint64(id.(float64)))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 999, "data": nil, "msg": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"code": 200, "data": nil, "msg": "success"})
		return
	})
	//拉取文件任务开启
	pullG.POST("/task_start", func(c *gin.Context) {
		req := make(map[string]interface{})
		c.BindJSON(&req)

		id, ok := req["task_id"]
		if !ok {
			c.JSON(http.StatusOK, gin.H{"code": 999, "data": nil, "msg": "task_id 不能为空"})
			return
		}
		path, ok := req["path"]
		if !ok {
			c.JSON(http.StatusOK, gin.H{"code": 999, "data": nil, "msg": "path 不能为空"})
			return
		}
		err := transferManger.PullTaskStart(uint64(id.(float64)), path.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 999, "data": nil, "msg": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"code": 200, "data": nil, "msg": "success"})
		return
	})
	//拉取文件任务删除
	pullG.POST("/task_del", func(c *gin.Context) {
		req := make(map[string]interface{})
		c.BindJSON(&req)

		id, ok := req["task_id"]
		if !ok {
			c.JSON(http.StatusOK, gin.H{"code": 999, "data": nil, "msg": "task_id 不能为空"})
			return
		}
		err := transferManger.PullTaskDel(uint64(id.(float64)))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 999, "data": nil, "msg": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"code": 200, "data": nil, "msg": "success"})
		return
	})

	return r
}

func RegisterMsgManger(r *connTcpMessage.Router) {
	//回复消息ID为0
	r.AddRouter(1, RecvNewPushTask)
	//r.AddRouter(2, RecvNewPushTask_rev)
}

func RecvNewPushTask(ctx context.Context, message *connTcpMessage.Message) ([]byte, error) {
	fmt.Println("收到消息：mid:", message.MID, " Data:", string(message.Data))
	return message.Data, nil
}
