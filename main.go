package main

import (
	//"errors"
	"fmt"
	//"io/fs"
	"io/ioutil"
	"os"
	"os/exec"

	"gopkg.in/yaml.v2"

	"flag"
	"strconv"
	"strings"
	"time"
)

type TestRun struct {
	Name                     string
	Worker_processes         string
	Worker_cpu_affinity      []string
	Worker_cpu_affinity_list []string

	Wrk_rate        string
	Wrk_threads     int
	Wrk_cpus        []string
	Wrk_connections string
	Wrk_file        string
}

// test config file
type TestConfig struct {
	Name        string
	Version     string
	Description string
	Runs        map[int]TestRun
}

func main() {
	fmt.Printf("Nginx tester version 1.0\n")
	testDir := flag.String("d", "", "test directory to run")
	help := flag.Bool("help", false, "Help")
	obj := TestConfig{}

	flag.Parse()

	if *help {
		flag.PrintDefaults()
		return
	}

	//check for tests directory
	path := "./tests"
	fileInfo, err := os.Stat(path)
	if err != nil {
		fmt.Printf("\x1B[31mno ./tests dir found\x1B[0m\n")
		return
	}

	if fileInfo.IsDir() {
		// is a directory
		fmt.Printf("./tests directory \x1B[32mfound\x1B[0m\n")
	} else {
		// is not a directory
		fmt.Printf("\x1B[31mno ./tests dir found\x1B[0m\n")
		return
	}
	//iterate through test directories
	files, err := os.ReadDir("./tests")
	if err != nil {
		fmt.Printf("could not read ./tests err %v\n", err)
		return
	}

	now := time.Now()
	//fmt.Println("Current date and time (RFC3339):", now.Format(time.RFC3339))
	//fmt.Println("Current date (YYYY-MM-DD):", now.Format("2006-01-02"))
	//fmt.Println("Current time (HH:MM:SS):", now.Format("15:04:05"))

	pathLogFile := fmt.Sprintf("./tests/log-%s-%s.txt", now.Format("2006-01-02"), now.Format("15:04:05"))
	logFile, err := os.Create(pathLogFile)
	if err != nil {
		fmt.Println(err)
		logFile.Close()
		return
	}
	defer logFile.Close()

	//fmt.Println("Log file created ", pathLogFile)

	for _, file := range files {
		if file.IsDir() {
			if len(*testDir) != 0 && file.Name() != *testDir {
				continue
			}
			//fmt.Printf("D %v\n", file.Name())
			//look for config.yaml file
			pathConfigFile := fmt.Sprintf("./tests/%v/config.yml", file.Name())
			yamlFile, err := ioutil.ReadFile(pathConfigFile)
			if err != nil {
				fmt.Printf("%v err   #%v \n", pathConfigFile, err)
				return
			}

			err = yaml.Unmarshal(yamlFile, &obj)
			if err != nil {
				fmt.Printf("Unmarshal: %v\n", err)
			}
			/*
				fmt.Printf("%v\n", obj)

				for i, run := range obj.Runs {
					fmt.Printf("Run[%d] %s\n", i, run.Name)
					fmt.Printf("        %v\n", run.Worker_cpu_affinity_list)
					fmt.Printf("        %s\n", run.Wrk_file)
				}
			*/
		}
	}

	runCnt := len(obj.Runs)
	for i := 1; i <= runCnt; i++ {
		run := obj.Runs[i]
		fmt.Printf("Run[%d] %s\n", i, run.Name)
		fmt.Printf("        %v\n", run.Worker_cpu_affinity_list)
		fmt.Printf("        %s\n", run.Wrk_file)

		fmt.Fprintf(logFile, "Run[%d] %s\n", i, run.Name)

		//build nginx config file
		_ = nginxConf(logFile, run)
		//restart xginx to read file
		//sudo service nginx restart
		cmd := exec.Command("service", "nginx", "restart")
		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("cmd.Run() failed with %s\n", err)
			fmt.Fprintf(logFile, "cmd.Run() failed with %s\n", err)
		}
		if len(out) > 0 {
			fmt.Printf("service nginx restart %d\n%s\n", len(out), out)
		}
		time.Sleep(2 * time.Second)

		//./wrk --rate 10000000000 -t 2 -C 8,9  -c 640 -d 60s https://localhost/1kb
		cpuCnt := len(run.Wrk_cpus)
		var cpuStr string
		for i := 0; i < cpuCnt; i++ {
			cpu := run.Wrk_cpus[i]
			if (i + 1) == cpuCnt {
				cpuStr += cpu
			} else {
				cpuStr += cpu
				cpuStr += ","
			}
		}
		//fmt.Printf("cpuStr >%s<\n", cpuStr)
		//fmt.Printf("connection %s\n", run.Wrk_connections)
		fmt.Fprintf(logFile, "/home/martin/amd_wrk2/wrk"+" "+"--rate"+" "+run.Wrk_rate+" "+"-t"+" "+strconv.Itoa(run.Wrk_threads)+" "+"-C"+" "+cpuStr+" "+"-c"+" "+run.Wrk_connections+" "+"-d"+" "+"60s"+" "+"--latency"+" "+run.Wrk_file)
		cmd = exec.Command("/home/martin/amd_wrk2/wrk", "--rate", run.Wrk_rate, "-t", strconv.Itoa(run.Wrk_threads), "-C", cpuStr, "-c", run.Wrk_connections, "-d", "60s", "--latency", run.Wrk_file)
		out, err = cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("./wrk failed with %s\n", err)
		}
		if len(out) > 0 {
			//fmt.Printf("out %d\n%s\n", len(out), out)
			extractResults(logFile, out)
		}

	}

}

/*
./wrk --rate 10000 -t 2 -C 8,9  -c 500 -d 60s --latency https://localhost/1kb
out 503
thtread 0 cpu 8
thtread 1 cpu 9
Running 1m test @ https://localhost/1kb
  2 threads and 500 connections
  Thread calibration: mean lat.: 3.541ms, rate sampling interval: 10ms
  Thread calibration: mean lat.: 4.224ms, rate sampling interval: 10ms
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency     2.56ms    1.68ms  54.27ms   87.78%
    Req/Sec     5.28k     4.67k   22.40k    63.73%
  Latency Distribution (HdrHistogram - Recorded Latency)
 50.000%    2.51ms
 75.000%    3.20ms
 90.000%    3.86ms
 99.000%    5.17ms
 99.900%   31.76ms
 99.990%   44.93ms
 99.999%   53.57ms
100.000%   54.30ms

#[Mean    =        2.561, StdDeviation   =        1.683]
#[Max     =       54.272, Total count    =       487497]
#[Buckets =           27, SubBuckets     =         2048]
----------------------------------------------------------
  589767 requests in 1.00m, 718.80MB read
Requests/sec:   9829.34
Transfer/sec:     11.98MB
*/

func extractResults(logFile *os.File, out []byte) {

	var tockens []string
	tockenStart := -1

	for i := 0; i < len(out); i++ {
		if out[i] > ' ' {
			if tockenStart < 0 {
				tockenStart = i
			}
			continue
		}
		out[i] = 0
		if tockenStart < 0 {
			continue
		}
		//str := string(out[6:])
		// fmt.Println(str)

		tockens = append(tockens, string(out[tockenStart:i]))
		tockenStart = -1

	}
	//for i := 0; i < len(tockens); i++ {
	//	fmt.Printf("%d %s\n", i, tockens[i])
	//}
	fmt.Fprintf(logFile, "\ncopy")
	for i := len(tockens) - 3; i < len(tockens); i++ {
		if tockens[i] == "Transfer/sec:" {
			fmt.Printf("Transfer/sec: %s\n", tockens[i+1])
			ss := strings.Split(tockens[i+1], "MB")
			if len(ss) == 2 {
				fmt.Printf("ss[0] %s\n", ss[0])
				fmt.Printf("ss[1] %s\n", ss[1])
				value, _ := strconv.ParseFloat(ss[0], 64)
				value = value * 1000000
				fmt.Printf("Transfer/sec: %s %.0f\n", tockens[i+1], value)
				fmt.Fprintf(logFile, "\t%.0f", value)

			} else {
				fmt.Printf("Transfer/sec: %s\n", tockens[i+1])
				fmt.Fprintf(logFile, "\t%s", tockens[i+1])
			}

			break
		}
	}

	for i := len(tockens) - 5; i < len(tockens); i++ {
		if tockens[i] == "Requests/sec:" {
			fmt.Printf("Requests/sec: %s\n", tockens[i+1])
			fmt.Fprintf(logFile, "\t%s", tockens[i+1])
			break
		}
	}
	for i := len(tockens) - 7; i < len(tockens); i++ {
		if tockens[i] == "read" {
			ss := strings.Split(tockens[i-1], "MB")
			if len(ss) == 2 {
				value, _ := strconv.ParseFloat(ss[0], 64)
				value = value * 1000000
				fmt.Printf("read %s %.0f\n", tockens[i-1], value)
				fmt.Fprintf(logFile, "\t%.0f", value)

			} else {
				ss := strings.Split(tockens[i-1], "GB")
				if len(ss) == 2 {
					value, _ := strconv.ParseFloat(ss[0], 64)
					value = value * 1000000000
					fmt.Printf("read %s %.0f\n", tockens[i-1], value)
					fmt.Fprintf(logFile, "\t%.0f", value)
				} else {
					fmt.Printf("read %s\n", tockens[i-1])
					fmt.Fprintf(logFile, "\t%s", tockens[i-1])
				}
			}
			break
		}
	}

	for i := 07; i < len(tockens); i++ {
		if tockens[i] == "50.000%" {
			for j := 0; j < 16; j = j + 2 {
				fmt.Printf("%s %s  ", tockens[i+j], tockens[i+j+1])
				ss := strings.Split(tockens[i+j+1], "ms")
				if len(ss) == 2 {
					fmt.Printf("%s\n", ss[0])
					fmt.Fprintf(logFile, "\t%s", ss[0])
				} else {
					ss = strings.Split(tockens[i+j+1], "s")
					if len(ss) == 2 {
						value, _ := strconv.ParseFloat(ss[0], 64)
						value = value * 1000
						fmt.Printf("%.0f\n", value)
						fmt.Fprintf(logFile, "\t%.0f", value)
					} else {
						ss = strings.Split(tockens[i+j+1], "m")
						if len(ss) == 2 {
							value, _ := strconv.ParseFloat(ss[0], 64)
							value = value * 60000
							fmt.Printf("%.0f\n", value)
							fmt.Fprintf(logFile, "\t%.0f", value)
						} else {
							fmt.Fprintf(logFile, "\t%s", tockens[i+j+1])
						}
					}
				}
			}
			break
		}
	}
	fmt.Fprintf(logFile, "\n")

}

func nginxConf(logFile *os.File, run TestRun) error {

	confFile, err := os.Create("/etc/nginx/nginx.conf")
	if err != nil {
		fmt.Println(err)
		confFile.Close()
		return err
	}
	defer confFile.Close()

	fmt.Fprintf(logFile, "/etc/nginx/nginx.conf\n")

	fmt.Fprintf(confFile, "worker_processes %s;\n", run.Worker_processes)
	fmt.Fprintf(logFile, "worker_processes %s;\n", run.Worker_processes)
	if len(run.Worker_cpu_affinity_list) > 0 {
		fmt.Fprintf(confFile, "worker_cpu_affinity_list ")
		fmt.Fprintf(logFile, "worker_cpu_affinity_list ")
		cpuCnt := len(run.Worker_cpu_affinity_list)
		for i := 0; i < cpuCnt; i++ {
			cpu := run.Worker_cpu_affinity_list[i]
			if (i + 1) == cpuCnt {
				fmt.Fprintf(confFile, "%s;\n", cpu)
				fmt.Fprintf(logFile, "%s;\n", cpu)
			} else {
				fmt.Fprintf(confFile, "%s,", cpu)
				fmt.Fprintf(logFile, "%s,", cpu)
			}
		}
	} else {
		args := len(run.Worker_cpu_affinity)
		if args == 1 {
			fmt.Fprintf(confFile, "worker_cpu_affinity %s;\n", run.Worker_cpu_affinity[0])
			fmt.Fprintf(logFile, "worker_cpu_affinity %s;\n", run.Worker_cpu_affinity[0])

		} else {
			fmt.Fprintf(confFile, "worker_cpu_affinity %s %s;\n", run.Worker_cpu_affinity[0], run.Worker_cpu_affinity[1])
			fmt.Fprintf(logFile, "worker_cpu_affinity %s %s;\n", run.Worker_cpu_affinity[0], run.Worker_cpu_affinity[1])
		}

	}

	fmt.Fprintf(confFile, "error_log   /var/log/nginx/error.log;\n")
	fmt.Fprintf(confFile, "events {\n\n}\n")
	fmt.Fprintf(confFile, "http {\n")
	fmt.Fprintf(confFile, "    include       mime.types;\n")
	fmt.Fprintf(confFile, "   default_type  application/octet-stream;\n")
	fmt.Fprintf(confFile, "     include /etc/nginx/conf.d/*.conf;\n}\n")

	return nil

}
