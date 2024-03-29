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
    LowWatermarkMaxSectors         int
    MiddleWatermarkMaxSectors      int
    HighWatermarkMaxSectors         int
    FlushMaxSectors                int
    LowWatermarkDirtyRatio         float64
    MiddleWatermarkDirtyRatio      float64
    HighWatermarkDirtyRatio        float64
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


    CONFIG.IncreaseMultiplier = c.IncreaseMultiplier
    CONFIG.DecreaseMultiplier = c.DecreaseMultiplier
    CONFIG.LowWatermarkMaxSectors = c.LowWatermarkMaxSectors
    CONFIG.MiddleWatermarkMaxSectors = c.MiddleWatermarkMaxSectors
    CONFIG.HighWatermarkMaxSectors = c.HighWatermarkMaxSectors
    CONFIG.FlushMaxSectors = c.FlushMaxSectors
    CONFIG.LowWatermarkDirtyRatio = c.LowWatermarkDirtyRatio
    CONFIG.MiddleWatermarkDirtyRatio = c.MiddleWatermarkDirtyRatio
    CONFIG.HighWatermarkDirtyRatio = c.HighWatermarkDirtyRatio
    CONFIG.MaxBcacheIoRate = c.MaxBcacheIoRate
    CONFIG.LogPath = c.LogPath
}

const (
	SYSFS_BLOCK = "/sys/block/"
	DISKSTATS   = "/proc/diskstats"
	STAT        = "/proc/stat"
    TOTAL       = 5
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
	//log.Printf("%s: Update writeback_rate_minimum to %d\r\n", devName, val)
}

func getMinWbRate(devName string) (val int) {
    path := SYSFS_BLOCK + devName + "/bcache/writeback_rate_minimum"

    if contents, err := ioutil.ReadFile(path); err == nil {
        result := strings.Replace(string(contents),"\n","",1)
        val, _ = strconv.Atoi(result)
    } else {
        panic(err)
    }

    return
}

func getCurrentDirtyRatio(devName string) (ratio float64) {
    path := SYSFS_BLOCK + devName + "/bcache/writeback_rate_debug"
    ratio = 0.0

    f, err := os.Open(path)
    if err != nil {
        log.Fatal(err)
    }
    defer func() {
        if err = f.Close(); err != nil {
        log.Fatal(err)
    }
    }()

    var d string
    var t string
    s := bufio.NewScanner(f)
    for s.Scan() {
        str := s.Text()
        if strings.HasPrefix(str, "dirty:") {
            as := strings.Fields(str)
            d = strings.ToLower(as[1])
        } else if strings.HasPrefix(str, "target:") {
            as := strings.Fields(str)
            t = strings.ToLower(as[1])
        }
    }

    if d != "" && t != "" {
        dirty := converter(d)
        target := converter(t)
        ratio = dirty / target
    }

    return
}

func converter(d string) (val float64) {
    if strings.HasSuffix(d, "k") {
        val, _ = strconv.ParseFloat(strings.Split(d, "k")[0], 64)
        val *= 1024
    } else if strings.HasSuffix(d, "m") {
        val, _ = strconv.ParseFloat(strings.Split(d, "m")[0], 64)
        val *= 1024 * 1024
    } else if strings.HasSuffix(d, "g") {
        val, _ = strconv.ParseFloat(strings.Split(d, "g")[0], 64)
        val *= 1024 * 1024 * 1024
    } else if strings.HasSuffix(d, "t") {
        val, _ = strconv.ParseFloat(strings.Split(d, "t")[0], 64)
        val *= 1024 * 1024* 1024 * 1024
    } else {
        val, _ = strconv.ParseFloat(d, 64)
    }

    return
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

func AdjustWorker(devName string, hData *HistoryData) {
	avg := hData.total
	avg.Div(hData.size)
    dirtyratio := getCurrentDirtyRatio(devName)
    currentRate := getMinWbRate(devName)

    if dirtyratio < CONFIG.LowWatermarkDirtyRatio {
        // case 1: dirtyratio < LowWatermarkDirtyRatio, we just set to LowWatermarkMaxSectors directly
        if currentRate != CONFIG.LowWatermarkMaxSectors {
            setMinWbRate(devName, CONFIG.LowWatermarkMaxSectors)
	        log.Printf("%s: LowWatermarkDirtyRatio: %.3f, set writeback rate to %d\n", devName, dirtyratio, CONFIG.LowWatermarkMaxSectors)
        } else {
	        log.Printf("%s: LowWatermarkDirtyRatio: %.3f, keep writeback rate to %d\n", devName, dirtyratio, CONFIG.LowWatermarkMaxSectors)
        }
    } else if dirtyratio < CONFIG.MiddleWatermarkDirtyRatio {
        // case 2: LowWatermarkDirtyRatio < dirtyratio <= MiddleWatermarkDirtyRatio, 
        var val float64 = float64(currentRate)
        var newvalue int
	    if avg.wPerSec < CONFIG.MaxBcacheIoRate && avg.rPerSec < CONFIG.MaxBcacheIoRate {
	        log.Printf("%s: MiddleWatermakeDirtyRatio, IDLE IO detected, avg: rPerSec:%.2f, wPerSec:%.2f, rkBPerSec:%.2f, wkBPerSec:%.2f, util:%.2f\n",
                       devName, avg.rPerSec, avg.wPerSec, avg.rkBPerSec, avg.wkBPerSec, avg.util)
	        newvalue = int(math.Ceil((val* CONFIG.IncreaseMultiplier)))
            if newvalue > CONFIG.MiddleWatermarkMaxSectors {
                newvalue = CONFIG.MiddleWatermarkMaxSectors
            }
        } else if avg.wPerSec > CONFIG.MaxBcacheIoRate || avg.rPerSec > CONFIG.MaxBcacheIoRate {
	        log.Printf("%s: MiddleWatermakeDirtyRatio, BUSY IO detected, avg: rPerSec:%.2f, wPerSec:%.2f, rkBPerSec:%.2f, wkBPerSec:%.2f, util:%.2f\n",
                       devName, avg.rPerSec, avg.wPerSec, avg.rkBPerSec, avg.wkBPerSec, avg.util)
	        newvalue = int(math.Floor(val / CONFIG.DecreaseMultiplier))

            if newvalue < CONFIG.LowWatermarkMaxSectors {
                newvalue = CONFIG.LowWatermarkMaxSectors
            }
        }

        if newvalue != currentRate {
            setMinWbRate(devName, newvalue)
	        log.Printf("%s: MiddleWatermarkDirtyRatio: %.3f, set writeback rate to %d\n", devName, dirtyratio, newvalue)
        } else {
	        log.Printf("%s: MiddleWatermarkDirtyRatio: %.3f, keep writeback rate to %d\n", devName, dirtyratio, newvalue)
        }
    } else if dirtyratio < CONFIG.HighWatermarkDirtyRatio {
        // case 3: MiddleWatermarkDirtyRatio < dirtyratio  < HighWatermarkDirtyRatio, we just set to LowWatermarkMaxSectors directly
        if currentRate != CONFIG.HighWatermarkMaxSectors {
            setMinWbRate(devName, CONFIG.HighWatermarkMaxSectors)
	        log.Printf("%s: HighWatermarkDirtyRatio: %.3f, set writeback rate to %d\n", devName, dirtyratio, CONFIG.HighWatermarkMaxSectors)
        } else {
	        log.Printf("%s: HighWatermarkDirtyRatio: %.3f, keep writeback rate to %d\n", devName, dirtyratio, CONFIG.HighWatermarkMaxSectors)
        }
    } else {
        // case 4: dirtyratio >HighWatermarkDirtyRatio, we just set to LowWatermarkMaxSectors directly
        if currentRate != CONFIG.FlushMaxSectors {
            setMinWbRate(devName, CONFIG.FlushMaxSectors)
	        log.Printf("%s: NearfullDirtyRatio: %.3f, set writeback rate to %d\n", devName, dirtyratio, CONFIG.FlushMaxSectors)
        } else {
	        log.Printf("%s: NearfullDirtyRatio: %.3f, keep writeback rate to %d\n", devName, dirtyratio, CONFIG.FlushMaxSectors)
        }
    }
    return
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

		    for name, hData := range hDataMap {
                AdjustWorker(name, hData)
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

    if !(CONFIG.LowWatermarkMaxSectors < CONFIG.MiddleWatermarkMaxSectors &&  CONFIG.LowWatermarkMaxSectors < CONFIG.HighWatermarkMaxSectors &&
         CONFIG.MiddleWatermarkMaxSectors < CONFIG.HighWatermarkMaxSectors) && CONFIG.HighWatermarkMaxSectors < CONFIG.FlushMaxSectors {
        log.Fatal("wrong LowWatermarkMaxSectors/MiddleWatermarkMaxSectors/HighWatermarkMaxSectors settings")
    }
    if !(CONFIG.LowWatermarkDirtyRatio < CONFIG.MiddleWatermarkDirtyRatio && CONFIG.LowWatermarkDirtyRatio < CONFIG.MiddleWatermarkDirtyRatio &&
        CONFIG.MiddleWatermarkDirtyRatio < CONFIG.HighWatermarkDirtyRatio)  {
        log.Fatal("wrong LowWatermarkDirtyRatio/MiddleWatermarkDirtyRatio/HighWatermarkDirtyRatio settings")
    }

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
