document.addEventListener('alpine:init', () => {
    Alpine.data('markdownEditor', () => ({
        content: '',
        isPreview: false,
        isFullScreen: false,
        isSplitView: false,
        isUploading: false, // 图片上传状态

        init() {
            // DOM-Bridge: Sync initial content from textarea value
            this.content = this.$refs.input.value;
            this.autoResize(this.$refs.input);
            console.log("Markdown Editor Initialized");
        },

        togglePreview() {
            this.isPreview = !this.isPreview;
            if (this.isPreview) this.isSplitView = false; // Disable split view if switching to full preview
        },

        toggleSplitView() {
            this.isSplitView = !this.isSplitView;
            if (this.isSplitView) {
                this.isPreview = false; // Ensure we are not in full preview mode
                // Force creating icons after layout change
                this.$nextTick(() => {
                    this.autoResize(this.$refs.input);
                });
            }
        },

        toggleFullScreen() {
            this.isFullScreen = !this.isFullScreen;
            if (!this.isFullScreen) {
                this.isSplitView = false; // Exit split view when exiting full screen
            }
            this.$nextTick(() => {
                lucide.createIcons();
                if (!this.isFullScreen && !this.isPreview) {
                    this.$refs.input.focus();
                }
            });
        },

        autoResize(el) {
            if (this.isFullScreen) return; // Fullscreen mode handles height via CSS
            el.style.height = 'auto';
            el.style.height = Math.max(200, el.scrollHeight) + 'px';
        },

        insertText(before, after) {
            const textarea = this.$refs.input;
            const start = textarea.selectionStart;
            const end = textarea.selectionEnd;
            const text = textarea.value; // Read directly from DOM to ensure sync

            const selectedText = text.substring(start, end);
            const newText = before + selectedText + after;

            // Update Alpine state
            this.content = text.substring(0, start) + newText + text.substring(end);

            // Restore cursor position after DOM update
            this.$nextTick(() => {
                textarea.focus();
                textarea.selectionStart = start + before.length;
                textarea.selectionEnd = start + before.length + selectedText.length;
                this.autoResize(textarea);
            });
        },

        renderMarkdown(text) {
            if (!text) return '<p class="text-stone-400 italic">预览区域 - 开始输入内容后这里会显示渲染效果...</p>';
            try {
                return marked.parse(text);
            } catch (e) {
                console.error("Markdown parse error:", e);
                return text;
            }
        },

        // ========== 图片上传相关功能 ==========

        // 触发文件选择对话框
        triggerImageUpload() {
            if (this.isUploading) return;
            this.$refs.imageInput.click();
        },

        // 处理文件选择上传
        async handleImageUpload(event) {
            const file = event.target.files[0];
            if (!file) return;
            await this.uploadAndInsertImage(file);
            // 重置 input，允许重复选择同一文件
            event.target.value = '';
        },

        // 处理粘贴事件
        async handlePaste(event) {
            const items = event.clipboardData?.items;
            if (!items) return;

            for (const item of items) {
                if (item.type.startsWith('image/')) {
                    event.preventDefault();
                    const file = item.getAsFile();
                    if (file) {
                        await this.uploadAndInsertImage(file);
                    }
                    return;
                }
            }
            // 非图片内容，让默认粘贴行为继续
        },

        // 上传并插入图片
        async uploadAndInsertImage(file) {
            if (this.isUploading) return;

            // 验证文件类型
            if (!file.type.startsWith('image/')) {
                alert('只能上传图片文件');
                return;
            }

            // 验证文件大小 (10MB)
            if (file.size > 10 * 1024 * 1024) {
                alert('图片大小不能超过 10MB');
                return;
            }

            this.isUploading = true;
            const textarea = this.$refs.input;
            const cursorPos = textarea.selectionStart;

            // 生成唯一占位符 ID
            const placeholderId = Date.now();
            const placeholder = `![上传中...(${placeholderId})]()`;

            // 在光标位置插入占位符
            this.insertTextAtCursor(placeholder);

            try {
                const formData = new FormData();
                formData.append('image', file);

                const response = await fetch('/api/upload', {
                    method: 'POST',
                    body: formData
                });

                const data = await response.json();

                if (!response.ok || !data.success) {
                    throw new Error(data.error || '上传失败');
                }

                // 生成图片文件名
                const fileName = file.name || 'image';
                const imageMarkdown = `![${fileName}](${data.url})`;

                // 替换占位符
                this.content = this.content.replace(placeholder, imageMarkdown);

            } catch (error) {
                console.error('图片上传失败:', error);
                // 移除占位符
                this.content = this.content.replace(placeholder, '');
                alert('图片上传失败: ' + (error.message || '请重试'));
            } finally {
                this.isUploading = false;
                this.$nextTick(() => {
                    this.autoResize(textarea);
                });
            }
        },

        // 在光标位置插入文本
        insertTextAtCursor(text) {
            const textarea = this.$refs.input;
            const start = textarea.selectionStart;
            const before = this.content.substring(0, start);
            const after = this.content.substring(start);
            this.content = before + text + after;

            this.$nextTick(() => {
                textarea.focus();
                textarea.selectionStart = textarea.selectionEnd = start + text.length;
            });
        }
    }));
});
