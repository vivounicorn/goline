package main

import (
	"fmt"
	"goline/server"
	"strconv"
	//	"io/ioutil"
	//	"goline/trainer"
	//	"os"
	//	"goline/util"
	"net/http"
	//	"os"
	//	"goline/util"
	"time"
)

func main() {
	//str := read("d:/test.dat")
	//	fmt.Print(str)
	//	util.SplitFile("d:/test.dat", 8)
	//	return

	//	client, err := hdfs.NewForUser("192.168.201.51:8020", "liyiran")
	//	if err != nil {
	//		fmt.Println(err)
	//		return
	//	}

	//	file, err := client.Open("/user/liyiran/ap-sample.txt")
	//	if err != nil {
	//		fmt.Println(err)
	//		return
	//	}
	//	buf := make([]byte, 59)
	//	file.ReadAt(buf, 48847)

	//	fmt.Println(string(buf))

	//	err = client.Get("/user/liyiran/test", "/data2/home/liyiran/go/merge.dat")
	//	if err != nil {
	//		fmt.Println(err)
	//		return
	//	}

	//	err = client.GetMerge("/user/liyiran/test/ap-sample1.txt", "/data2/home/liyiran/go/ap-sample1.txt", true)
	//	if err != nil {
	//		fmt.Println(err)
	//		return
	//	}

	//	util.FileSample("D:\\test.dat", 1024*1024)
	//	return

	//模型训练
	//	var fft trainer.FastFtrlTrainer
	//	fft.Initialize(1, 8, false, 0, 10, 10)
	//	fft.Train(0.1, 1, 10, 10, 0.1, "D:\\model.dat",
	//		"D:\\train.dat",
	//		"D:\\train.dat")

	//	return

	//src := "D:\\liyirango\\src\\goline\\demo\\model3\\off\\train.dat"
	//util.FileSample(src, 23630559, 1024*1024*1024)

	a, _ := strconv.ParseFloat("-1", 64)
	fmt.Println(int(a))
	return

	plugin := &server.Lands{}
	err := plugin.Initialize("..\\conf\\settings.conf")
	if err != nil {
		fmt.Println(err)
	}
	server := http.Server{
		Addr:        ":8080",
		Handler:     plugin,
		ReadTimeout: 6 * time.Second,
	}

	server.ListenAndServe()
}
