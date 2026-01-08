package services

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ImgurResponse Imgur API 响应结构
type ImgurResponse struct {
	Data struct {
		ID         string `json:"id"`
		Link       string `json:"link"`
		DeleteHash string `json:"deletehash"`
		Type       string `json:"type"`
	} `json:"data"`
	Success bool `json:"success"`
	Status  int  `json:"status"`
}

// ImageUploadResult 上传结果
type ImageUploadResult struct {
	URL         string `json:"url"`          // 反代链接
	OriginalURL string `json:"original_url"` // 原始 Imgur 链接
	ID          string `json:"id"`           // 图片 ID
}

// UploadToImgur 上传图片到 Imgur
// 参数: file - multipart 文件
// 返回: ImageUploadResult, error
func UploadToImgur(file multipart.File, header *multipart.FileHeader) (*ImageUploadResult, error) {
	clientID := os.Getenv("IMGUR_CLIENT_ID")
	if clientID == "" {
		return nil, fmt.Errorf("IMGUR_CLIENT_ID 未配置")
	}

	// 读取文件内容
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	// 转换为 base64
	base64Image := base64.StdEncoding.EncodeToString(fileBytes)

	// 使用 multipart/form-data 格式构建请求体
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	// 添加 image 字段
	if err := writer.WriteField("image", base64Image); err != nil {
		return nil, fmt.Errorf("写入请求体失败: %w", err)
	}

	// 添加 type 字段，指明是 base64 格式
	if err := writer.WriteField("type", "base64"); err != nil {
		return nil, fmt.Errorf("写入请求体失败: %w", err)
	}

	writer.Close()

	req, err := http.NewRequest("POST", "https://api.imgur.com/3/image", &requestBody)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Authorization", "Client-ID "+clientID)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// 发送请求
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("上传请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 解析响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var imgurResp ImgurResponse
	if err := json.Unmarshal(body, &imgurResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if !imgurResp.Success {
		return nil, fmt.Errorf("Imgur 上传失败: status %d", imgurResp.Status)
	}

	// 获取文件扩展名
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext == "" {
		// 根据 MIME 类型推断扩展名
		switch imgurResp.Data.Type {
		case "image/jpeg":
			ext = ".jpg"
		case "image/png":
			ext = ".png"
		case "image/gif":
			ext = ".gif"
		case "image/webp":
			ext = ".webp"
		default:
			ext = ".jpg"
		}
	}

	// 返回结果，使用反代链接
	return &ImageUploadResult{
		URL:         fmt.Sprintf("/img/%s%s", imgurResp.Data.ID, ext),
		OriginalURL: imgurResp.Data.Link,
		ID:          imgurResp.Data.ID,
	}, nil
}

// UploadImage 通用上传接口（未来可切换到其他服务）
// 当前默认使用 Imgur
func UploadImage(file multipart.File, header *multipart.FileHeader) (*ImageUploadResult, error) {
	return UploadToImgur(file, header)
}
