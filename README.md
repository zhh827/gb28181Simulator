使用go语言开发的GB28181模拟器,在[原作者GIT地址](https://github.com/lzh2nix/gb28181Simulator)代码基础上重构,**目前还在开发中**,计划增加功能如下:
- [ ] 多会话管理
- [ ] 直接使用mp4/flv文件,虚拟IPC
- [ ] 使用本地USB摄像头,虚拟IPC
- [ ] socket接收rtp流,虚拟IPC
- [ ] 作为上级平台,允许nvr注册到本平台
- [ ] 上下级平台视频流中转

### 启动

```bash
go run main.go -c sim.conf
```

### 配置文件
```json
{
  "localSipPort": 5061,
  "serverID": "32011500002000000001",
  "realm": "3201150000",
  "serverAddr": "127.0.0.1:5061",
  "userName": "test",
  "password": "test",
  "regExpire": 3600,
  "keepaliveInterval": 100,
  "maxKeepaliveRetry": 3,
  "transport": "udp",
  "gbId": "31011500991320000532",
  "devices": [
    {
      "deviceID": "32011500991320000040",
      "name": "test001",
      "manufacturer": "simulatorFactory",
      "model": "Mars",
      "CivilCode": "civilCode",
      "address": "192.18.1.1",
      "parental": "0",
      "safeWay": "1",
      "registerWay": "1",
      "secrecy": "1",
      "status": "ON"
    },
    {
      "deviceID": "32011500991320000041",
      "name": "test002",
      "manufacturer": "simulatorFactory",
      "model": "Mars",
      "CivilCode": "civilCode",
      "address": "192.18.1.2",
      "parental": "0",
      "safeWay": "1",
      "registerWay": "1",
      "secrecy": "1",
      "status": "ON"
    }
  ]
}
```
**配置说明**:
|          Property              |              Description                  |
|:------------------------------:|:-----------------------------------------:|
|          localSipPort          |              gb28181 本地端口              |
|            serverID            |               server 国标ID                |
|              realm             |               server 国标域               |
|           serverAddr           |      server 服务器地址(接入服务地址)      |
|            userName            |          国标用户名(从服务端获取)         |
|            password            |           国标密码(从服务端获取)          |
|            regExpire           |              设备注册超时时间             |
|        keepaliveInterval       |             keepalive 发送间隔            |
|        maxKeepaliveRetry       | keeplive超时次数(超时之后发送重新发送reg) |
|            transport           |         传输层协议(目前只支持udp)         |
|              gbId              |                 设备国标ID                |
|        devices.deviceID        |                子设备国标ID               |
|          devices.name          |                 子设备名称                |
|      devices.manufacturer      |                 子设备厂商                |
|          devices.model         |                子设备model                |
|         devices.address        |                子设备ip地址               |
|         devices.status         |                 子设备状态                |
