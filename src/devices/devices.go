// package devices 主要处理和设备相关的通信

package devices

import (
	"comm"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	online  = uint(0)
	offline = uint(1)
	noData  = uint(2)
)

//Device 设备
type Device struct {
	hardwareID   uint
	port         uint
	hardwareCode string
	//devType      string
	conn  net.Conn
	state uint //当前状态 0 正常  1 网络断开 2 设备不能返回数据
	isOk  uint //命令执行结果
}

// DevType 设备类型
type DevType struct {
	url     string
	devlist []uint
}

var devList = make(map[uint]*Device, 100) //设备列表

var devTypeTable = make(map[string][]uint, 10) //设备列表的类型索引，值为该类型的所有设备
var reqDevListTicker = time.NewTicker(time.Minute * 2)
var reportStatusTicker = time.NewTicker(time.Second * 10)

/*
TADIAO-001= 塔吊
YANGCHEN-001= 扬尘监测
DIANTI-001  =  电梯
RFID-001 = RFID识别器
SHUIBIA-O001 = 智能水表
DIANBIAO-001= 智能电表
WUSHUI-001= 污水监测
DIBANG-001 = 地磅
SHEXIANGTOU-001 = 摄像头
*/

func getDevType(dev string) (result string, err error) {
	devType := strings.Split(dev, "-")[0]
	switch devType {
	case "DIANBIAO":
		result = "电表"
	case "SHUIBIAO":
		result = "水表"
	case "TADIAO":
		result = "塔吊"
	case "WUSHUI":
		result = "污水"
	case "ENV":
		result = "环境"
	case "ZAOYIN":
		result = "噪音"
	case "RFID":
		result = "RFID"
	case "DIANTI":
		result = "电梯"
	case "DIBANG":
		result = "地磅"
	case "SHEXIANGTOU":
		result = "摄像头"
	default:
		err = errors.New("设备类型不存在")
	}
	return result, err
}
func initDevTypeTbale() {
	devTypeTable["电表"] = make([]uint, 0, 5)
	devTypeTable["水表"] = make([]uint, 0, 5)
	devTypeTable["塔吊"] = make([]uint, 0, 5)
	devTypeTable["污水"] = make([]uint, 0, 5)
	devTypeTable["环境"] = make([]uint, 0, 5)
	devTypeTable["RFID"] = make([]uint, 0, 5)
	devTypeTable["电梯"] = make([]uint, 0, 5)
	devTypeTable["地磅"] = make([]uint, 0, 5)
	devTypeTable["摄像头"] = make([]uint, 0, 5)
}

func relayError(id string, errType string) {
	//json := generateDataJSONStr(id, "ERROR", errType)
	//sendData(urlTable["错误"], id, []byte(json))
}

var urlTable = map[string]string{
	"电表":   "http://39.108.5.184/smart/api/saveElectricityData",
	"水表":   "http://39.108.5.184/smart/api/saveWaterData",
	"塔吊":   "http://39.108.5.184/smart/api/saveCraneData",
	"污水":   "http://39.108.5.184/smart-api/api/checkIn",
	"环境":   "http://39.108.5.184/smart/api/saveEnvData",
	"RFID": "http://39.108.5.184/smart/api/checkIn",
	"电梯":   "http://39.108.5.184/smart/api/saveElevatorData",
	"地磅":   "",
	"摄像头":  "",
	"设备列表": "http://39.108.5.184/smart/api/getHardwareList?projectId=1",
	"设备状态": "http://39.108.5.184/smart/api/reportState"}

// GetURL 获取要发消息的url
func GetURL(urlStr string) (url string) {
	return urlTable[urlStr]
}

// GetConn 通过ID获取当前链接
/*func getConn(id string) net.Conn {
	devConnTable := findDevConnTbale(id)
	return devConnTable[id]
}
*/
// BindConn 绑定连接到具体设备 并设定状态为上线
func bindConn(id uint, conn net.Conn) {
	devList[id].conn = conn
	devList[id].state = online
}

// UnBindConn 解除设备的连接绑定 并设定状态为断开
func unBindConn(id uint) {
	devList[id].conn = nil
	devList[id].state = offline
}

func setStateNoData(id uint) {
	if devList[id].state == online {
		devList[id].state = noData
	}
}
func setStateOk(id uint) {
	if devList[id].state == noData {
		devList[id].state = online
	}
}

// reqDevList 向服务器请求设备列表
/*{ "code":200, "data":[ { "area":"生活区", "hardwareCode":"DIANBIAO-001", "hardwareId":1, "name":"智能电表", "port":10001 }, { "area":"施工区", "hardwareCode":"DIANBIAO-002", "hardwareId":2, "name":"智能电表", "port":10002 }, { "area":"大门", "hardwareCode":"RFID-001", "hardwareId":3, "name":"RFID读卡器", "port":10003 } ], "errMsg":"" } */

func reqDevList(url string) error {
	//sendServ([]byte(`{"MsgType":"Serv","Action":"DevList"}`))
	//fmt.Printf("reqDevList start\n")
	type jsonDev struct {
		Area         string `json:"area"`
		HardwareCode string `json:"hardwareCode"`
		HardwareID   uint   `json:"hardwareId"`
		Name         string `json:"name"`
		Port         uint   `json:"port"`
	}
	type jsonDevList struct {
		Code   int       `json:"code"`
		Data   []jsonDev `json:"data"`
		ErrMsg string    `json:"errMsg"`
	}
	var reqDevListData jsonDevList
	//http://39.108.5.184/smart/api/getHardwareList?projectId=1

	/*client := &http.Client{
		Transport: &http.Transport{
			Dial: func(netw, addr string) (net.Conn, error) {
				conn, err := net.DialTimeout(netw, addr, time.Second*2)
				if err != nil {
					return nil, err
				}
				conn.SetDeadline(time.Now().Add(time.Second * 2))
				return conn, nil
			},
			ResponseHeaderTimeout: time.Second * 2,
		},
	}*/
	fmt.Printf("reqDevList HTTP GET\n")
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("获取设备列表错误：%s\n", err.Error())
		return err
	}
	defer resp.Body.Close()
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("获取设备列表内容读取失败：%s\n", err.Error())
		return err
	}
	fmt.Printf("获取设备列表内容:n%s\n", string(content))
	err = json.Unmarshal(content, &reqDevListData)
	if err != nil {
		log.Printf("解析设备列表json数据失败：%s\n", err.Error())
		return err
	}

	fmt.Println(reqDevListData)
	if reqDevListData.Code != 200 {
		log.Printf("服务器错误\n")
		err := errors.New("服务器错误")
		return err
	}
	for _, v := range reqDevListData.Data {
		var dev = new(Device)
		devTypeStr, err := getDevType(v.HardwareCode)
		if err != nil {
			log.Printf("%s不存在的类型\n", v.HardwareCode)
			continue
		}
		//列表中不存在则加入列表
		if _, ok := devList[v.HardwareID]; !ok {
			dev.port = v.Port
			dev.hardwareCode = v.HardwareCode
			dev.hardwareID = v.HardwareID
			dev.conn = nil
			dev.state = offline
			dev.isOk = 1
			devList[dev.hardwareID] = dev
			devTypeTable[devTypeStr] = append(devTypeTable[devTypeStr], dev.hardwareID)

			//创建新的监听并等待连接
			port := strconv.FormatUint(uint64(dev.port), 10)
			listen, err := net.Listen("tcp", "localhost:"+port)
			if err != nil {
				log.Printf("监听失败:%s,%s\n", "localhost:"+port, err.Error())
				listen.Close()
				continue
			}
			if listen == nil {
				log.Println("listen == nil")
				continue
			}
			fmt.Printf("监听 【%s】成功\n", port)
			go devAcceptConn(listen, dev.hardwareID)
		}
	}
	fmt.Println(devList)
	return nil
}
func getConn(id uint) net.Conn {
	if _, ok := devList[id]; ok {
		return devList[id].conn
	}
	return nil
}

// devAcceptConn 等待设备连接，创建连接
func devAcceptConn(l net.Listener, hardwareID uint) {
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Printf("监听建立连接错误:%s\n", err.Error())
			l.Close()
			continue
		}
		fmt.Printf("建立连接成功:%d\n", hardwareID)
		bindConn(hardwareID, conn)
		devTypeStr, _ := getDevType(devList[hardwareID].hardwareCode)
		if "塔吊" == devTypeStr {
			taDiaoStart(hardwareID)
		}
	}
}

//把获取的设备数据分装到json中
func generateDataJSONStr(id string, action string, data string) string {
	str := fmt.Sprintf(`{"MsgType":"Devices","ID":"%s","Action":"%s","Data":"%s"}`, id, action, data)
	return str
}

func reportDevStatus() {
	for _, dev := range devList {
		devState := make(url.Values, 4)
		devState["isOk"] = []string{strconv.FormatInt(int64(dev.isOk), 10)}
		devState["state"] = []string{strconv.FormatInt(int64(dev.state), 10)}
		sendData("设备状态", dev.hardwareID, devState)
	}
}
func sendData(urlStr string, id uint, data url.Values) {
	var msg comm.MsgData
	msg.SetTime()
	msg.HdID = id
	msg.Data = data
	msg.URLStr = urlStr
	comm.SendMsg(msg)
}

// IntiDevice 初始化设备连接
func IntiDevice() error {
	initDevTypeTbale()
	//定时请求设备列表
	go func() {
		reqDevList(urlTable["设备列表"])
		for _ = range reqDevListTicker.C {
			reqDevList(urlTable["设备列表"])
		}
	}()
	//定时上报状态
	go func() {
		for _ = range reportStatusTicker.C {
			reportDevStatus()
		}
	}()
	//塔吊是主动上报 只能被动接收 在创建连接的时候就建立读协程等待数据
	dianBiaoIntAutoGet() //启动电表数据获取
	shuiBiaoAutoGet()    //启动水表数据获取
	wuShuiAutoGet()      //启动污水数据获取
	return nil
}
