# file_transfer_manager






# API 说明文档
## 1. 接口：/v1/push/task_new
**功能**：发送文件给一个节点

**请求参数**：
```json
{
  "to_addr": "0.0.0.0:8089",
  "path": "E:\\xin\\流程图.rar"
}
```
**请求参数说明**：
- path - 文件路径，不能为空
- to_addr - 接收ip+端口，不能为空

**返回参数**：
```json
{
  "code": 200,
  "data": {
    "push_task_id": 5,
    "pull_task_id": 5,
    "to": "0.0.0.0:8089",
    "path": "E:\\xin\\流程图.rar",
    "hash": "V2O99wJdMM9QCYVO+Hleon+qWB/Wbja9oAzXUCorTiE=",
    "expiration_time": 1687773051
  },
  "msg": "success"
}
```
**返回参数说明**：
- code - 200成功，其它为失败
- push_task_id - push端任务id
- pull_task_id - pull端任务id
- to - 文件发到的接收方地址
- path - 文件路径
- expiration_time - 任务过期时间，push端发起的任务有时间期限



## 2. 接口：/v1/push/sharing_dirs
**功能**：获取共享目录列表

**请求参数**：无

**返回参数**：
```json
{
  "code": 200,
  "data": {
    "share_dir": "E:\\zb\\share_dir",
    "xin": "E:\\xin"
  },
  "msg": "success"
}
```

**返回参数说明**：
- data - key为目录key，pull端有权限拉取以"key/子目录/文件"的路径去拉取文件，所以在添加共享目录时，最后一级目录名不能重复


## 3. 接口：/v1/push/sharing_dirs_add
**功能**：添加共享目录

**请求参数**：
```json
{
  "dir": "E:\\xin"
}
```
**请求参数说明**：
- dir - 目录，不能为空，最后一级目录名不能重复，并且最后一级目录名不要是根盘符

**返回参数**：
```json
{
"code": 200,
"data": null,
"msg": "success"
}
```

## 4. 接口：/v1/push/sharing_dirs_del
**功能**：删除共享目录
**请求参数**：
```json
{
"dir": "目录"
}
```
**返回参数**：
```json
{
"code": 200,
"data": null,
"msg": "success"
}
```
**参数说明**：
- dir - 目录，不能为空
## 5. 接口：/v1/push/pull_addr_whitelist
**功能**：获取授权白名单地址列表

**请求参数**：无

**返回参数**：
```json
{
  "code": 200,
  "data": [
    "0.0.0.0:8089"
  ],
  "msg": "success"
}
```
## 6. 接口：/v1/push/pull_addr_whitelist_add
**功能**：添加授权白名单地址

**请求参数**：
```json
{
"addr": "0.0.0.0:8089"
}
```
**返回参数**：
```json
{
"code": 200,
"data": null,
"msg": "success"
}
```
**参数说明**：
- addr - 地址，不能为，只有在白名单的地址才能有权限向自己发送pull任务
## 7. 接口：/v1/push/pull_addr_whitelist_del
**功能**：删除授权白名单地址
**请求参数**：
```json
{
"addr": "0.0.0.0:8089"
}
```
**返回参数**：
```json
{
"code": 200,
"data": null,
"msg": "success"
}
```
**参数说明**：
- addr - 地址，不能为空
## 8. 接口：/v1/pull/task_new
**功能**：向一个节点发起拉取任务

**请求参数**：
```json
{
  "source": "xin/流程图.rar",
  "path": "E:\\dow\\流程图.rar",
  "from_addr": "0.0.0.0:8088"
}
```
**请求参数说明**：
- source - 源文件，不能为空，目录的根为push端的共享目录的key
- path - 路径，这里path可选，为空者文件保存到常量recfilepath设置的目录下
- from_addr - 来源地址，不能为空，push端地址

**返回参数**：
```json
{
"code": 200,
"data": null,
"msg": "success"
}
```

## 9. 接口：/v1/pull/task_is_auto_set
**功能**：设置是否自动拉取

**请求参数**：
```json
{
"auto": true/false
}
```
**返回参数**：
```json
{
"code": 200,
"data": null,
"msg": "success"
}
```
**参数说明**：
- auto - 是否自动拉取，不能为空,设置自动拉取为ture后，push端发起过来的任务将自动开始传输文件，否则任务将为pendingConfirmation(待确认)状态，需要手动开启pull任务
## 10. 接口：/v1/pull/task_is_auto_get
**功能**：获取是否自动拉取状态

**请求参数**：无

**返回参数**：
```json
{
"code": 200,
"data": true/false,
"msg": "success"
}
```
## 11. 接口：/v1/pull/task_list
**功能**：获取拉取文件任务列表

**请求参数**：无

**返回参数**：
```json
{
  "code": 200,
  "data": [
    {
      "push_task_id": 4,
      "pull_task_id": 4,
      "file_info": {
        "from": "0.0.0.0:8088",
        "to": "0.0.0.0:8089",
        "name": "流程图.rar",
        "hash": "V2O99wJdMM9QCYVO+Hleon+qWB/Wbja9oAzXUCorTiE=",
        "size": 275487629,
        "path": "files\\流程图.rar",
        "index": 250572800,
        "data": null,
        "speed": {
          "size": 5473267,
          "time": 1687685696
        },
        "rate": 90
      },
      "status": "stop",
      "fault": ""
    },
    {
      "push_task_id": 5,
      "pull_task_id": 5,
      "file_info": {
        "from": "0.0.0.0:8088",
        "to": "0.0.0.0:8089",
        "name": "流程图.rar",
        "hash": "V2O99wJdMM9QCYVO+Hleon+qWB/Wbja9oAzXUCorTiE=",
        "size": 275487629,
        "path": "files\\流程图.rar",
        "index": 21606400,
        "data": null,
        "speed": {
          "size": 15188154,
          "time": 1687686655
        },
        "rate": 7
      },
      "status": "stop",
      "fault": ""
    },
    {
      "push_task_id": 8,
      "pull_task_id": 8,
      "file_info": {
        "from": "0.0.0.0:8088",
        "to": "0.0.0.0:8089",
        "name": "流程图.rar",
        "hash": "V2O99wJdMM9QCYVO+Hleon+qWB/Wbja9oAzXUCorTiE=",
        "size": 275487629,
        "path": "E:\\dow\\流程图.rar",
        "index": 58163200,
        "data": null,
        "speed": {
          "size": 273659,
          "time": 1687747087
        },
        "rate": 21
      },
      "status": "running",
      "fault": ""
    }
  ],
  "msg": "success"
}
```
**返回参数说明**：
- code - 200成功，其它为失败
- push_task_id - push端任务id
- pull_task_id - pull端任务id
- from - 文件来源端地址
- to - 文件发到的接收方地址
- name - 文件名
- hash - 文件hash
- size - 文件大小
- path - 文件保存到的路径
- index - 文件下一次需要拿取的偏移位置
- data - 传输中的数据（这里不显示）
- speed - 传输采集速度参数（用于计算传输速度）
- rate - 已经传输百分比
- status - 任务状态，pendingConfirmation待确认、running运行中、stop已停止
- fault - 传输文件过程中的异常

## 12. 接口：/v1/pull/task_stop
**功能**：停止拉取文件任务
**请求参数**：
```json
{
  "task_id": 5
}
```
**返回参数**：
```json
{
"code": 200,
"data": null,
"msg": "success"
}
```
**参数说明**：
- task_id - pull任务ID，不能为空
## 13. 接口：/v1/pull/task_start
**功能**：开启拉取文件任务

**请求参数**：
```json
{
"task_id": "pull任务ID",
"path": "路径"
}
```

**请求参数说明**：
- task_id - pull任务ID，不能为空
- path - 路径，可选，任务状态为pendingConfirmation(待确认)可以设置文件保存的路径，不设置将保存到常量recfilepath设置的目录下，任务其它状态时该参数不起效

**返回参数**：
```json
{
"code": 200,
"data": null,
"msg": "success"
}
```

## 14. 接口：/v1/pull/task_del
**功能**：删除拉取文件任务
**请求参数**：
```json
{
"task_id": "pull任务ID"
}
```
**返回参数**：
```json
{
"code": 200,
"data": null,
"msg": "success"
}
```
**参数说明**：
- task_id - pull任务ID，不能为空
















## p2p文件传输管理器流程图

![流程图](./p2p文件传输管理器.jpg)





