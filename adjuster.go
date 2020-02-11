package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
	"math"
)

type config struct {
    IncreaseMultiplier             float64
    DecreaseMultiplier             float64
    MaxWritebackSectors            int
    MinWritebackSectors            int
    MaxBcacheIoRate                float64
    LogPath                        string
}

// use global variable
var CONFIG config

func setupConfig() {
    f, err := os.Open("/etc/adjuster/config.json")
    if err != nil {
        panic("Cannot open config file")
    }
    defer f.Close()

    var c config
    err = json.NewDecoder(f).Decode(&c)
    if err != nil {
        panic("Failed to parse config.json: " + err.Error())
    }

    // setup CONFIG with defaults
    CONFIG.IncreaseMultiplier = c.IncreaseMultiplier
    CONFIG.DecreaseMultiplier = c.DecreaseMultiplier
    CONFIG.MaxWritebackSectors = c.MaxWritebackSectors
    CONFIG.LogPath = c.LogPath
    CONFIG.MaxBcacheIoRate = c.MaxBcacheIoRate
}

const (
	SYSFS_BLOCK = "/sys/block/"
	DISKSTATS   = "/proc/diskstats"
	STAT        = "/proc/stat"
	TOTAL       = 10
)

type IoStats struct {
	rdSectors uint64
	wrSectors uint64
	rdIos     uint64
	wrIos     uint64
	rdTicks   uint32
	wrTicks   uint32
	iosPgr    uint32
	totTicks  uint32
	rqTicks   uint32
}

type DevStats struct {
	name    string
	iostats IoStats
}

type DevsStats struct {
	time     time.Time
	devStats []DevStats
}

type ExtStats struct {
	rPerSec   float64
	wPerSec   float64
	rkBPerSec float64
	wkBPerSec float64
	util      float64
}

type HistoryData struct {
	total    ExtStats
	curr     int32
	size     int32
	extStats [TOTAL]ExtStats
}

var minVar map[string]int

func (e *ExtStats) Add(extStats ExtStats) error {
	e.rkBPerSec += extStats.rkBPerSec
	e.wkBPerSec += extStats.wkBPerSec
	e.rPerSec += extStats.rPerSec
	e.wPerSec += extStats.wPerSec
	e.util += extStats.util
	return nil
}

func (e *ExtStats) Div(divisor int32) error {
	e.rkBPerSec /= float64(divisor)
	e.wkBPerSec /= float64(divisor)
	e.rPerSec /= float64(divisor)
	e.wPerSec /= float64(divisor)
	e.util /= float64(divisor)
	return nil
}

func (e *ExtStats) Sub(extStats ExtStats) error {
	e.rkBPerSec -= extStats.rkBPerSec
	e.wkBPerSec -= extStats.wkBPerSec
	e.rPerSec -= extStats.rPerSec
	e.wPerSec -= extStats.wPerSec
	e.util -= extStats.util
	return nil
}

func setMinWbRate(devName string, val int) {
	path := SYSFS_BLOCK + devName + "/bcache/writeback_rate_minimum"

	file, err := os.OpenFile(path, os.O_WRONLY, 0755)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	s := strconv.Itoa(val)
	file.Write([]byte([]byte(s)))
	log.Printf("Update writeback_rate_minimum of device %s to %d\r\n", devName, val)
}

func getMinWbRate(devName string) (val int) {
    path := SYSFS_BLOCK + devName + "/bcache/writeback_rate_minimum"

    if contents, err := ioutil.ReadFile(path); err == nil {
        result := strings.Replace(string(contents),"\n","",1)
        val, _ = strconv.Atoi(result)
	    //log.Println("current writeback_rate_minimum is ", val)
    } else {
        panic(err)
    }

    return
}

func updateMinRate(devName string, shouldInc bool, shouldDec bool) {
    if (shouldInc == false && shouldDec == false) || (shouldInc == true && shouldDec == true) {
       log.Println("shouldInc and shouldDec is not correct")
       return
    }

    if minVar == nil {
    	minVar = make(map[string]int)
    }
    if _, ok := minVar[devName]; !ok {
    minVar[devName] = getMinWbRate(devName)
    }

    var newvalue int
    var val float64 = float64(minVar[devName])

    if shouldInc == true {
	    val = math.Ceil((val * CONFIG.IncreaseMultiplier))
    }
    if shouldDec == true {
	    val = math.Ceil(val / CONFIG.DecreaseMultiplier) 
    }
    newvalue = int(val)

    if newvalue > CONFIG.MaxWritebackSectors {
        newvalue = CONFIG.MaxWritebackSectors
    }

    if newvalue < CONFIG.MinWritebackSectors {
        newvalue = CONFIG.MinWritebackSectors
    }

    if minVar[devName] != newvalue {
        minVar[devName] = newvalue
        setMinWbRate(devName, newvalue)
    } else {
        log.Printf("keep writeback rate of %s to %d unchanged\r\n", devName, newvalue)
    }
}

func isBcacheDevice(name string) bool {
    if strings.HasPrefix(name, "bcache") {
        return true
    }
    return false
}

func readDiskstatsStat(devsStats *DevsStats) error {
	var major, minor int32
	var iosPgr, totTicks, rqTicks, wrTicks uint32
	var rdIos, rdMergesOrRdSec, rdTicksOrWrSec, wrIos uint64
	var wrMerges, rdSecOrWrIos, wrSec uint64
	var dcIos, dcMerges, dcSec, dcTicks uint64
	var devName string

	file, err := os.Open(DISKSTATS)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	for {
		line, _, err := reader.ReadLine()
		if err == io.EOF {
			break
		}
		/*fmt.Printf("%s \n", line)*/
		i, _ := fmt.Sscanf(string(line), "%d %d %s %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d",
			&major, &minor, &devName,
			&rdIos, &rdMergesOrRdSec, &rdSecOrWrIos, &rdTicksOrWrSec,
			&wrIos, &wrMerges, &wrSec, &wrTicks, &iosPgr, &totTicks, &rqTicks,
			&dcIos, &dcMerges, &dcSec, &dcTicks)

		/*scan dev list*/
		if !isBcacheDevice(devName) {
			continue
		}
		/*
			log.Printf("major:%d, minor:%d, devName:%s,rdIos:%d,rdMergesOrRdSec:%d,rdSecOrWrIos:%d,"+
				"rdTicksOrWrSec:%d, wrIos:%d, wrMerges:%d, wrSec:%d, wrTicks:%d, iosPgr:%d, totTicks:%d, rqTicks:%d,"+
				"dcIos:%d,dcMerges:%d,dcSec:%d,dcTicks:%d\n", major, minor, devName, rdIos, rdMergesOrRdSec, rdSecOrWrIos,
				rdTicksOrWrSec, wrIos, wrMerges, wrSec, wrTicks, iosPgr, totTicks, rqTicks,
				dcIos, dcMerges, dcSec, dcTicks)
		*/
		if i >= 14 {
			var devStats DevStats

			devStats.iostats.rdIos = rdIos
			devStats.iostats.rdSectors = rdSecOrWrIos
			devStats.iostats.rdTicks = uint32(rdTicksOrWrSec)
			devStats.iostats.wrIos = wrIos
			devStats.iostats.wrSectors = wrSec
			devStats.iostats.wrTicks = wrTicks
			devStats.iostats.iosPgr = iosPgr
			devStats.iostats.totTicks = totTicks
			devStats.iostats.rqTicks = rqTicks

			devStats.name = devName
			devsStats.devStats = append(devsStats.devStats, devStats)
		} else {
			/*Unknown entry: Ignore it*/
		}
	}

	return nil
}

func shouldAdjust(name string, hData *HistoryData) (shouldInc bool, shouldDec bool) {
	avg := hData.total
	avg.Div(hData.size)

	if avg.wPerSec < CONFIG.MaxBcacheIoRate && avg.rPerSec < CONFIG.MaxBcacheIoRate {
	    log.Printf("IDLE IO detected on device %s: avg: rPerSec:%.2f, wPerSec:%.2f, rkBPerSec:%.2f, wkBPerSec:%.2f, util:%.2f\n",
		            name, avg.rPerSec, avg.wPerSec, avg.rkBPerSec, avg.wkBPerSec, avg.util)
        return true, false
    }

    if avg.wPerSec > CONFIG.MaxBcacheIoRate || avg.rPerSec > CONFIG.MaxBcacheIoRate {
	    log.Printf("BUSY IO detected on device %s: avg: rPerSec:%.2f, wPerSec:%.2f, rkBPerSec:%.2f, wkBPerSec:%.2f, util:%.2f\n",
	    	        name, avg.rPerSec, avg.wPerSec, avg.rkBPerSec, avg.wkBPerSec, avg.util)
        return false, true
    }
    return false, false
}

func processStats(ch chan DevsStats) error {
	sdevMap := make(map[string]*DevStats)
	hDataMap := make(map[string]*HistoryData)

	var prevtime time.Time
	for {
		devs := <-ch
		for _, curr := range devs.devStats {
			var prev *DevStats
			var ok bool

			name := curr.name
			if prev, ok = sdevMap[name]; !ok {
				prev = new(DevStats)
				sdevMap[name] = prev
				prev.name = name
				prev.iostats = curr.iostats

				continue
			}

			var extStats ExtStats
			delta := devs.time.Sub(prevtime)
			extStats.util = (float64(curr.iostats.totTicks) - float64(prev.iostats.totTicks)) / float64(delta/time.Millisecond) * 100
			extStats.rPerSec = (float64(curr.iostats.rdIos) - float64(prev.iostats.rdIos))/float64(delta/time.Second)
			extStats.wPerSec = (float64(curr.iostats.wrIos) - float64(prev.iostats.wrIos))/float64(delta/time.Second)
			extStats.rkBPerSec = (float64(curr.iostats.rdSectors) - float64(prev.iostats.rdSectors))/float64(delta/time.Second)
			extStats.rkBPerSec = extStats.rkBPerSec / 2
			extStats.wkBPerSec = (float64(curr.iostats.wrSectors) - float64(prev.iostats.wrSectors))/float64(delta/time.Second)
			extStats.wkBPerSec = extStats.wkBPerSec / 2

			var hData *HistoryData
			if hData, ok = hDataMap[name]; !ok {
				hData = new(HistoryData)
				hDataMap[name] = hData
			}

			if hData.size == 0 {
				hData.extStats[hData.curr] = extStats
				hData.size += 1

				hData.total.Add(extStats)
			} else if hData.size == TOTAL {
				hData.curr = (hData.curr + 1) % TOTAL
				/*delete last element*/
				deleted := hData.extStats[hData.curr]
				hData.total.Sub(deleted)

				hData.extStats[hData.curr] = extStats
				hData.total.Add(extStats)
			} else {
				hData.curr = (hData.curr + 1) % TOTAL
				hData.extStats[hData.curr] = extStats
				hData.size += 1

				hData.total.Add(extStats)
			}

			prev.name = name
			prev.iostats = curr.iostats

			//log.Printf("curr: name:%s, rPerSec:%.2f, wPerSec:%.2f, rkBPerSec:%.2f, wkBPerSec:%.2f, util:%.2f\n",
            // 		name, extStats.rPerSec, extStats.wPerSec, extStats.rkBPerSec, extStats.wkBPerSec, extStats.util)

		    for name, hData := range hDataMap {
                if inc, dec := shouldAdjust(name, hData); (inc == true && dec == false) || (inc == false && dec == true){
                    updateMinRate(name, inc, dec)
                }
		    }

		}


		prevtime = devs.time
	}

	return nil
}

func main() {
    setupConfig()
	interval := 1 * time.Second


	f, err := os.OpenFile(CONFIG.LogPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()
	log.SetOutput(f)

	ch := make(chan DevsStats, 64)
	go processStats(ch)

	for {
		now := time.Now()
		var devsStats DevsStats
		err := readDiskstatsStat(&devsStats)
		if err != nil {
			log.Fatalln("Error reading disk stats! err: ", err)
		}
        if len(devsStats.devStats) > 0 {
		    devsStats.time = now
		    ch <- devsStats
        } else {
	        log.Println("no bcach edeivce detected this time, wait for next one")
        }

		time.Sleep(interval)
	}
}
