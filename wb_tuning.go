package main

import (
	"bufio"
	"errors"
	"flag"
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

const (
	SYSFS_BLOCK = "/sys/block/"
	DISKSTATS   = "/proc/diskstats"
	STAT        = "/proc/stat"
	TOTAL       = 10
)

var (
	cdevs   = flag.String("devs", "", "Cache device list, eg: cache0,cache1")
	logpath = flag.String("log", "./wb.log", "Log file path name")
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
	idx      int
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
	log.Println("Set writeback_rate_minimum ", s)
}

func getMinWbRate(devName string) (val int) {
    path := SYSFS_BLOCK + devName + "/bcache/writeback_rate_minimum"

    if contents, err := ioutil.ReadFile(path); err == nil {
        result := strings.Replace(string(contents),"\n","",1)
        val, _ = strconv.Atoi(result)
	    log.Println("get current writeback_rate_minimum ", val)
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

    if (shouldInc && minVar[devName] >= 8192 ) || (shouldDec && minVar[devName] <= 10) {
       log.Printf("keep the value %d unchanged\r\n", minVar[devName])
        return
    }

    var val float64 = float64(minVar[devName])

    if shouldInc == true {
	    val = math.Ceil((val * 1.1))
    }
    if shouldDec == true {
	    val = math.Ceil(val / 1.1)
    }

    if int(val) >= 8192 {
        val = 8192
    }

    if int(val) <= 10 {
        val = 10
    }

    if minVar[devName] != int(val) {
        minVar[devName] = int(val)
        setMinWbRate(devName, int(val))
    }
}

func checkDeviceName(devList []string) error {
	for _, name := range devList {
		var path string
		if strings.HasPrefix(name, "/dev/") {
			path = name
			name = strings.TrimPrefix(name, "/dev/")
		} else {
			path = "/dev/" + name
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return errors.New("Cache dev " + name + " not exist!")
		}
	}
	return nil
}

func readDiskstatsStat(devList []string, devsStats *DevsStats) error {
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
		var exist bool
		for _, dev := range devList {
			if devName == dev {
				exist = true
				break
			}
		}
		if !exist {
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

func shouldAdjust(hData *HistoryData) (shouldInc bool, shouldDec bool) {
	avg := hData.total
	avg.Div(hData.size)
	log.Printf("avg: rPerSec:%.2f, wPerSec:%.2f, rkBPerSec:%.2f, wkBPerSec:%.2f, util:%.2f\n",
		avg.rPerSec, avg.wPerSec, avg.rkBPerSec, avg.wkBPerSec, avg.util)

	if avg.wPerSec < 100 && avg.rPerSec < 100 {
        return true, false
    }

    if avg.wPerSec > 100 || avg.rPerSec > 100 {
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

			log.Printf("curr: name:%s, rPerSec:%.2f, wPerSec:%.2f, rkBPerSec:%.2f, wkBPerSec:%.2f, util:%.2f\n",
				name, extStats.rPerSec, extStats.wPerSec, extStats.rkBPerSec, extStats.wkBPerSec, extStats.util)

		    for name, hData := range hDataMap {
                if inc, dec := shouldAdjust(hData); (inc == true && dec == false) || (inc == false && dec == true){
                    updateMinRate(name, inc, dec)
                }
		    }

		}


		prevtime = devs.time
	}

	return nil
}

func main() {
	interval := 1 * time.Second
	var devList []string
	count := 0

	flag.Parse()
	if *cdevs == "" {
		log.Println("Error cache devs parameter! cdevs:", *cdevs)
		return
	}
	devList = strings.Split(*cdevs, ",")
	if devList == nil {
		log.Println("Error sparsing dev list!")
		return
	}
	err := checkDeviceName(devList)
	if err != nil {
		log.Println(err)
		return
	}

	f, err := os.OpenFile(*logpath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
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
		err := readDiskstatsStat(devList, &devsStats)
		if err != nil {
			log.Fatalln("Error reading disk stats! err: ", err)
		}
		if len(devsStats.devStats) != len(devList) {
			log.Fatalln("Error getting ", len(devList), "disk stats")
		}
		devsStats.time = now
		devsStats.idx = count
		ch <- devsStats

		count = (count + 1) % 2
		time.Sleep(interval)
	}
}
