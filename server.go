package main
 
import (
    "net/http"
    "fmt"
    "flag"
)
 
func main(){
	var filePath string
	var port string
	flag.StringVar(&filePath,"f", "/data/tegcode/data/", "文件服务器的文件路径")
	flag.StringVar(&port,"p", "7777", "服务器的端口号")

	flag.Parse()

	fmt.Println("server start successfully")
	
    http.Handle("/", http.FileServer(http.Dir(filePath)))
    http.ListenAndServe(":"+port, nil)
    
}

