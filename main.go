package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/mail"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// 程序主函数，执行入口。
func main() {
	// tcp协议，监听在25端口。
	ln, err := net.Listen("tcp", ":25")
	if err != nil {
		panic(err)
	}
	//程序退出时，关闭监听的端口。
	defer ln.Close()
	//在终端输出提示。
	fmt.Println("Server listening on :25")
	//进入无限循环。
	for {
		//如果有请求，就创建连接，并返回连接 conn ；否则就阻塞等待客户端的连接。
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}
		//启动协程处理连接请求。
		go handleConnection(conn)
	}
}

// 处理客户的请求
func handleConnection(conn net.Conn) {
	//本函数执行完毕后，关闭连接。
	defer conn.Close()

	//对conn进行封装，分别用于数据的读取和写入。
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	//打印日志到终端
	fmt.Println("接收到来自", conn.RemoteAddr(), "的连接")

	//建立连接后，服务端主动响应客户端 220 代码
	fmt.Fprintf(writer, "220 %s SMTP service ready.\r\n", "naive mail")
	writer.Flush()

	//检查 From 的正则表达式。
	fromPattern := `(?i:From):\s*<([^>]+)>`
	fromRe, err := regexp.Compile(fromPattern)
	if err != nil {
		fmt.Println("正则表达式编译错误:", err)
		return
	}
	//检查 To 的正则表达式。
	toPattern := `(?i:To):\s*<([^>]+)>`
	toRe, err := regexp.Compile(toPattern)
	if err != nil {
		fmt.Println("正则表达式编译错误:", err)
		return
	}

	var from string         //保存 from 数据,只能有一个 from 。
	var to []string         //保存 to 数据，允许多个to 。
	var buffer bytes.Buffer //保存文件内容数据。
	//开始进入接收数据
	for {
		//读一行，以回车分割的行
		message, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading message:", err)
			break
		}
		//打印接收的客户端命令(数据)到终端
		fmt.Print("Received message: ", message)
		//去掉客户端命令结尾的换行符和回车符
		message = strings.TrimSpace(message)
		//解析命令,以空格为分隔符，分为两段。
		cmdWithParams := strings.SplitN(message, " ", 2)
		//第一部分是命令，四个字符，全部转为大写。
		cmd := strings.ToUpper(cmdWithParams[0])
		//准备执行后的返回代码和回复的消息变量
		var code int
		var msg string
		//读取一行数据，解析命令结果，进行匹配执行：
		switch cmd {
		case "HELO":
			//忽略 HELO 携带的参数
			code = 250
			msg = "Hello, welcome to naive mail!"
			fmt.Fprintf(writer, "%d %s\r\n", code, msg)
			writer.Flush()
		case "MAIL":
			//解析参数
			//命令原型为: Mail From:myName@mydomain.com
			params := cmdWithParams[1]
			if len(strings.TrimSpace(params)) == 0 {
				code = 501
				msg = "Invisible user are not welcome!"
			} else {
				//发送者实际允许有多个，受客户端的影响，一般只能填写一个。
				if !strings.HasPrefix(strings.ToUpper(params), "FROM:") {
					code = 500
					msg = "5.5.2 Unknown command"
				}
				var addrs []string
				// 查找所有匹配项
				matches := fromRe.FindAllStringSubmatch(params, -1)
				for _, match := range matches {
					if len(match) > 1 {
						fmt.Println("捕获的邮箱地址:", match[1])
						addrs = append(addrs, match[1])
					}
				}
				//如果有多个From,只取第一个
				from = addrs[0]
				code = 250
				msg = "OK"
			}
			fmt.Fprintf(writer, "%d %s\r\n", code, msg)
			writer.Flush()
		case "EHLO":
			// 无参数
			buf := bytes.NewBuffer(nil)
			fmt.Fprint(buf, "naive mail - It is OK.\n")
			fmt.Fprint(buf, "8BITMIME\n")
			fmt.Fprint(buf, "PIPELINING\n")
			fmt.Fprint(buf, "SMTPUTF8\n")
			fmt.Fprint(buf, "AUTH LOGIN PLAIN CRAM-MD5\n")
			fmt.Fprintf(buf, "SIZE %d\n", 1000)
			fmt.Fprint(buf, "STARTTLS\n")
			fmt.Fprint(buf, "HELP\n")
			code = 250
			msg = buf.String()
			//以换行符分割消息:这种情况只针对 EHLO 返回多行的情况进行处理，因此在 EHLO 返回数据构造时，多行数据需要使用 \n 进行分隔，
			// 其他命令返回单行的无需在返回数据结尾添加 \n 回车符。
			lines := strings.Split(msg, "\n")
			//计数
			var i int
			//前n-1个回复code和消息之间包含 - 符号
			for i = 0; i < len(lines)-2; i++ {
				fmt.Fprintf(writer, "%d-%s\r\n", code, lines[i])
			}
			//最后一个回复code和消息之间不包含 - 符号
			fmt.Fprintf(writer, "%d %s\r\n", code, lines[i])
			writer.Flush()
		case "HELP":
			// 无参数
			code = 214
			msg = "no help and support"
			fmt.Fprintf(writer, "%d %s\r\n", code, msg)
			writer.Flush()
		case "NOOP":
			// 无参数
			code = 250
			msg = "OK"
			fmt.Fprintf(writer, "%d %s\r\n", code, msg)
			writer.Flush()
		case "RSET":
			// 无参数
			from = ""
			to = nil
			buffer.Reset()
			code = 250
			msg = "OK"
			fmt.Fprintf(writer, "%d %s\r\n", code, msg)
			writer.Flush()
		case "VRFY":
			// 无参数
			code = 502
			msg = "no support"
			fmt.Fprintf(writer, "%d %s\r\n", code, msg)
			writer.Flush()
		case "EXPN":
			// 无参数
			code = 502
			msg = "no support"
			fmt.Fprintf(writer, "%d %s\r\n", code, msg)
			writer.Flush()
		case "RCPT":
			//解析参数
			params := cmdWithParams[1]
			if len(strings.TrimSpace(params)) == 0 {
				code = 501
				msg = "Invisible user are not welcome!"
			} else {
				//发送者可能有多个
				if !strings.HasPrefix(strings.ToUpper(params), "To:") {
					code = 500
					msg = "5.5.2 Unknown command"
				}
				var addrs []string
				// 查找所有匹配项
				matches := toRe.FindAllStringSubmatch(params, -1)
				fmt.Println("邮箱地址:", params)
				for _, match := range matches {
					if len(match) > 1 {
						fmt.Println("捕获的邮箱地址:", match[1])
						addrs = append(addrs, match[1])
					}
				}
				to = addrs
				code = 250
				msg = "OK"
			}
			fmt.Fprintf(writer, "%d %s\r\n", code, msg)
			writer.Flush()
		case "DATA":
			//接收邮件数据
			//输出信息到客户端，同意传输数据
			code = 354
			msg = "Enter mail, end with . on a line by itself"
			fmt.Fprintf(writer, "%d %s\r\n", code, msg)
			writer.Flush()
			// 开始读取邮件数据
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					fmt.Println("Error reading message:", err)
					break
				}
				//收到数据结束标记
				if line == ".\r\n" {
					break
				}

				buffer.WriteString(line)
			}

			//在这里可以直接保存到.eml文件里面。
			// 创建emails目录
			if _, err := os.Stat("emails"); os.IsNotExist(err) {
				os.Mkdir("emails", 0755)
			}

			filename := fmt.Sprintf("emails/email_%d.eml", time.Now().UnixNano())
			file, err := os.Create(filename)
			if err != nil {
				fmt.Println("Error reading message:", err)
				break
			}
			defer file.Close()
			//========================================================================
			// 构建完整的邮件头
			// 解析邮件数据
			emailContent, err := mail.ReadMessage(bytes.NewReader(buffer.Bytes()))
			if err != nil {
				code = 451
				msg = "Error in processing email"
			}
			// 邮件内容中没有包含From信息
			bodyFrom := emailContent.Header.Get("From")
			if bodyFrom == "" {
				emailContent.Header["From"] = []string{from}
			}
			// 邮件内容中没有包含To信息
			bodyto := emailContent.Header.Get("to")
			if bodyto == "" {
				emailContent.Header["to"] = to
			}
			// 邮件内容中没有包含Subject信息
			bodySubject := emailContent.Header.Get("Subject")
			if bodySubject == "" {
				emailContent.Header["Subject"] = []string{"Subject: [SMTP Server] Message received at %s\r\n", time.Now().Format("2006-01-02 15:04:05")}
			}

			// 添加发送方服务器IP
			receivedHeader := fmt.Sprintf("Received: from %s by %s (SMTP Server); %s\r\n",
				"unkown", conn.RemoteAddr(), time.Now().Format(time.RFC1123Z))

			// 添加日期
			dateHeader := fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z))

			//添加消息ID
			messageIDHeader := fmt.Sprintf("Message-ID: <%s@%s>\r\n", fmt.Sprintf("%d.%d", time.Now().UnixNano(), os.Getpid()), conn.RemoteAddr())

			_, err = file.WriteString(receivedHeader + dateHeader + messageIDHeader + buffer.String())
			//=========================================================================

			// 输出到终端显示
			fmt.Printf("收到邮件:\n发件人: %s\n收件人: %v\n主题: %s\n", from, to, emailContent.Header.Get("Subject"))

			// 处理邮件数据，独立保存附件。
			// 检查是否为多部分邮件
			contentType := emailContent.Header.Get("Content-Type")
			if strings.Contains(contentType, "multipart/") {
				// 获取边界
				mediaType, params, err := mime.ParseMediaType(emailContent.Header.Get("Content-Type"))
				if err != nil {
					code = 451
					msg = "Error in processing email"
				}
				if !strings.HasPrefix(mediaType, "multipart/") {
					code = 451
					msg = "Error in processing email"
					fmt.Print("不是多部分邮件")
				}
				boundary, ok := params["boundary"]
				if !ok {
					code = 451
					msg = "Error in processing email"
					fmt.Print("未找到边界")
				}
				// 解析附件各个部分数据
				mr := multipart.NewReader(emailContent.Body, boundary)
				partIndex := 0

				for {
					part, err := mr.NextPart()
					if err == io.EOF {
						break
					}
					if err != nil {
						fmt.Println("Error parse multipart body:", err)
						break
					}

					contentType := part.Header.Get("Content-Type")
					contentDisposition := part.Header.Get("Content-Disposition")

					// 检查是否为附件
					if strings.Contains(contentDisposition, "attachment") ||
						strings.Contains(contentType, "application/") ||
						(strings.Contains(contentDisposition, "inline") && part.FileName() != "") {

						filename := part.FileName()
						if filename == "" {
							_, params, _ := mime.ParseMediaType(contentDisposition)
							filename = params["filename"]
						}

						fmt.Printf("发现附件: %s (类型: %s)\n", filename, contentType)

						// 读取附件内容
						content, err := io.ReadAll(part)
						if err != nil {
							fmt.Println("Error parse multipart body:", err)
							break
						}

						// 如果是base64编码，需要解码
						if strings.Contains(part.Header.Get("Content-Transfer-Encoding"), "base64") {
							decoded := make([]byte, base64.StdEncoding.DecodedLen(len(content)))
							n, err := base64.StdEncoding.Decode(decoded, content)
							if err != nil {
								fmt.Println("Error parse multipart body:", err)
								break
							}
							content = decoded[:n]
						}

						// 保存附件
						attachmentDir := "attachments"
						if err := os.MkdirAll(attachmentDir, 0755); err != nil {
							fmt.Println("Error create attachment dir:", err)
							break
						}
						// 生成唯一文件名
						timestamp := time.Now().Format("20060102_150405")
						uniqueFilename := fmt.Sprintf("%s_%d_%s", timestamp, partIndex, filename)
						filePath := filepath.Join(attachmentDir, uniqueFilename)

						// 保存文件
						if err := os.WriteFile(filePath, content, 0644); err != nil {
							fmt.Println("Error save attachment file:", err)
							break
						}

						fmt.Printf("附件已保存: %s (大小: %d 字节)\n", filePath, len(content))

						partIndex++
					} else {
						// 处理文本内容
						content, err := io.ReadAll(part)
						if err != nil {
							fmt.Println("Error parse multipart body:", err)
							break
						}
						fmt.Printf("文本内容: %s\n", string(content))
					}

					part.Close()
				}
			} else {
				// 处理纯文本邮件
				body, _ := io.ReadAll(emailContent.Body)
				fmt.Printf("邮件内容: %s\n", string(body))
			}
			//写完后，清空数据变量。
			from = ""
			to = nil
			buffer.Reset()
			code = 250
			msg = "Message sent"
			fmt.Fprintf(writer, "%d %s\r\n", code, msg)
			writer.Flush()
		case "STARTTLS":
			code = 250
			msg = "OK"
			fmt.Fprintf(writer, "%d %s\r\n", code, msg)
			writer.Flush()
		case "AUTH":
			code = 250
			msg = "OK"
			fmt.Fprintf(writer, "%d %s\r\n", code, msg)
			writer.Flush()
		case "QUIT":
			code = 221
			msg = "BYE"
			fmt.Fprintf(writer, "%d %s\r\n", code, msg)
			writer.Flush()
		case "GET", "POST", "CONNECT":
			code = 250
			msg = "OK"
			fmt.Fprintf(writer, "%d %s\r\n", code, msg)
			writer.Flush()
		default:
			cmd = fmt.Sprintf("unknown<%.6q>", cmd)
			code = 500
			msg = "5.5.1 Unknown command"
			fmt.Fprintf(writer, "%d %s\r\n", code, msg)
			writer.Flush()
		}
	}
}
