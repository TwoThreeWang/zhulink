// Markdown Editor Component for Alpine.js

// 预览与服务端渲染保持一致：goldmark 开了 WithHardWraps、不输出原始 HTML，bluemonday 只放行 http/https/mailto 链接
if (typeof marked !== 'undefined') {
    marked.use({
        breaks: true,
        renderer: {
            html() { return ''; }
        },
        walkTokens(token) {
            if ((token.type === 'link' || token.type === 'image') && !isSafeURL(token.href)) {
                token.href = '#';
            }
        }
    });
}

// 用浏览器自己的 URL 解析判断协议，能识别 "java\tscript:" 这类变形
function isSafeURL(href) {
    try {
        return ['http:', 'https:', 'mailto:'].includes(new URL(href, location.href).protocol);
    } catch (e) {
        return false;
    }
}

function escapeHTML(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// 定义组件工厂函数
function markdownEditor() {
    return {
        previewHTML: '',
        isPreview: false,
        isFullScreen: false,
        isSplitView: false,
        isUploading: false, // 图片上传状态
        scrollSource: null, // 对照模式下当前由哪一栏驱动滚动：'input' | 'preview'
        lastLength: 0, // 上次内容长度，用来判断输入框是否需要缩小
        previewTimer: 0,
        syncFrame: 0,

        init() {
            this.lastLength = this.$refs.input.value.length;
            this.autoResize(this.$refs.input, true);
        },

        get previewVisible() {
            return this.isPreview || this.isSplitView;
        },

        togglePreview() {
            this.isPreview = !this.isPreview;
            if (this.isPreview) {
                this.isSplitView = false; // Disable split view if switching to full preview
                this.updatePreview();
            }
        },

        toggleSplitView() {
            this.isSplitView = !this.isSplitView;
            if (this.isSplitView) {
                this.isPreview = false; // Ensure we are not in full preview mode
                this.updatePreview();
                this.$nextTick(() => {
                    this.autoResize(this.$refs.input);
                    // 打开对照时让预览对齐到当前编辑位置
                    this.scrollSource = 'input';
                    this.syncScroll('input');
                });
            }
        },

        toggleFullScreen() {
            this.isFullScreen = !this.isFullScreen;
            if (!this.isFullScreen) {
                this.isSplitView = false; // Exit split view when exiting full screen
            }
            this.$nextTick(() => {
                // 进入全屏清掉行内高度交给 CSS；退出全屏按内容重新计算高度
                this.autoResize(this.$refs.input, true);
                if (!this.isFullScreen && !this.isPreview) {
                    this.$refs.input.focus({ preventScroll: true });
                }
            });
        },

        onInput() {
            const length = this.$refs.input.value.length;
            this.autoResize(this.$refs.input, length < this.lastLength);
            this.lastLength = length;
            this.schedulePreview();
            this.syncScroll('input');
        },

        // shrink=false 时只在内容超出时撑高，避免每次按键都把高度设为 auto 触发两次整页重排
        autoResize(el, shrink = false) {
            // 全屏时高度由 CSS 撑满，textarea 内部滚动；残留的行内高度会把它卡在 200px
            if (this.isFullScreen) {
                el.style.height = '';
                return;
            }
            if (!shrink) {
                if (el.scrollHeight > el.clientHeight) el.style.height = el.scrollHeight + 'px';
                return;
            }
            // 设为 auto 的瞬间页面变短，浏览器会把滚动位置截断到顶部附近，所以要记住并恢复
            const scrollY = window.scrollY;
            el.style.height = 'auto';
            el.style.height = Math.max(200, el.scrollHeight) + 'px';
            window.scrollTo(0, scrollY);
        },

        // 对照模式同步滚动：只让鼠标/键盘所在的一栏驱动另一栏，避免两边互相触发；每帧最多算一次
        syncScroll(from) {
            if (!this.isSplitView || this.scrollSource !== from || this.syncFrame) return;
            this.syncFrame = requestAnimationFrame(() => {
                this.syncFrame = 0;
                const src = from === 'input' ? this.$refs.input : this.$refs.preview;
                const dst = from === 'input' ? this.$refs.preview : this.$refs.input;
                const max = src.scrollHeight - src.clientHeight;
                // ponytail: 按滚动比例同步，图片/代码块很多时两边会有偏差；要精确需按源码行号给预览块建锚点
                dst.scrollTop = max > 0 ? src.scrollTop / max * (dst.scrollHeight - dst.clientHeight) : 0;
            });
        },

        // ========== 预览：只在可见时渲染，输入停顿后再渲染 ==========

        schedulePreview() {
            if (!this.previewVisible) return;
            clearTimeout(this.previewTimer);
            this.previewTimer = setTimeout(() => this.updatePreview(), 150);
        },

        updatePreview() {
            clearTimeout(this.previewTimer);
            this.previewHTML = this.renderMarkdown(this.$refs.input.value);
            if (this.isSplitView) this.$nextTick(() => this.syncScroll('input'));
        },

        renderMarkdown(text) {
            if (!text) return '<p class="text-stone-400 italic">预览区域 - 开始输入内容后这里会显示渲染效果...</p>';
            try {
                return marked.parse(text);
            } catch (e) {
                console.error("Markdown parse error:", e);
                return `<pre>${escapeHTML(text)}</pre>`;
            }
        },

        // ========== 文本编辑 ==========

        // 用 execCommand 插入以保留浏览器原生撤销记录（GitHub 等编辑器同做法），不支持时退回 setRangeText
        replaceRange(start, end, text, selStart = start + text.length, selEnd = selStart) {
            const textarea = this.$refs.input;
            textarea.focus({ preventScroll: true });
            textarea.setSelectionRange(start, end);
            const ok = text
                ? document.execCommand('insertText', false, text)
                : start === end || document.execCommand('delete');
            if (!ok) {
                textarea.setRangeText(text, start, end);
                textarea.dispatchEvent(new Event('input', { bubbles: true }));
            }
            textarea.setSelectionRange(selStart, selEnd);
        },

        // 行内格式（加粗、斜体等）：包住选区；已被同样标记包住时再点一次则去掉
        insertText(before, after) {
            const textarea = this.$refs.input;
            const { selectionStart: start, selectionEnd: end, value } = textarea;
            const selected = value.slice(start, end);

            // 标记在选区外侧，如 **|abc|**；多看一位避免把 **粗体** 的一个 * 当成斜体
            if (value.slice(start - before.length, start) === before && value.slice(end, end + after.length) === after
                && value[start - before.length - 1] !== before[0]) {
                this.replaceRange(start - before.length, end + after.length, selected,
                    start - before.length, end - before.length);
                return;
            }
            // 标记在选区内侧，如 |**abc**|
            if (selected.length >= before.length + after.length && selected.startsWith(before) && selected.endsWith(after)) {
                const inner = selected.slice(before.length, selected.length - after.length);
                this.replaceRange(start, end, inner, start, start + inner.length);
                return;
            }
            this.replaceRange(start, end, before + selected + after,
                start + before.length, start + before.length + selected.length);
        },

        // 行首格式（标题、引用、列表）：作用于选区覆盖的每一行，全部已有前缀时再点一次则去掉
        prefixLines(prefix) {
            const textarea = this.$refs.input;
            const { selectionStart: start, selectionEnd: end, value } = textarea;
            const ordered = prefix === '1. ';
            const pattern = ordered ? /^\d+\. / : null;
            const hasPrefix = line => ordered ? pattern.test(line) : line.startsWith(prefix);

            const lineStart = value.lastIndexOf('\n', start - 1) + 1;
            // 选区刚好结束在换行后时，不算上下一行
            let lineEnd = value.indexOf('\n', end > start && value[end - 1] === '\n' ? end - 1 : end);
            if (lineEnd < 0) lineEnd = value.length;

            const block = value.slice(lineStart, lineEnd);
            const lines = block.split('\n');
            const filled = lines.filter(line => line.trim());
            const remove = filled.length > 0 && filled.every(hasPrefix);
            let n = 0;
            const result = lines.map(line => {
                if (!line.trim() && lines.length > 1) return line;
                if (remove) return ordered ? line.replace(pattern, '') : line.slice(prefix.length);
                return (ordered ? `${++n}. ` : prefix) + line;
            }).join('\n');

            if (start === end) {
                const pos = Math.max(lineStart, start + result.length - block.length);
                this.replaceRange(lineStart, lineEnd, result, pos, pos);
            } else {
                this.replaceRange(lineStart, lineEnd, result, lineStart, lineStart + result.length);
            }
        },

        // 块级内容（代码块、分割线）前面补足空行：紧贴文字的 --- 会把上一行变成标题
        blockPrefix(pos) {
            const before = this.$refs.input.value.slice(0, pos);
            if (!before || before.endsWith('\n\n')) return '';
            return before.endsWith('\n') ? '\n' : '\n\n';
        },

        insertCodeBlock() {
            const { selectionStart: start, selectionEnd: end } = this.$refs.input;
            this.insertTextAround(this.blockPrefix(start) + '```\n', '\n```\n', start, end);
        },

        insertRule() {
            const { selectionStart: start } = this.$refs.input;
            this.replaceRange(start, start, this.blockPrefix(start) + '---\n\n');
        },

        insertTextAround(before, after, start, end) {
            const selected = this.$refs.input.value.slice(start, end);
            this.replaceRange(start, end, before + selected + after,
                start + before.length, start + before.length + selected.length);
        },

        // 链接：选中的是网址则放进括号、光标停在文字处；否则选中 url 占位方便直接输入
        insertLink() {
            const { selectionStart: start, selectionEnd: end, value } = this.$refs.input;
            const selected = value.slice(start, end);
            if (/^https?:\/\/\S+$/.test(selected)) {
                this.replaceRange(start, end, `[](${selected})`, start + 1);
                return;
            }
            const text = `[${selected}](url)`;
            this.replaceRange(start, end, text, start + selected.length + 3, start + text.length - 1);
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

        // 处理粘贴事件：只有纯图片（如截图）才上传；
        // Office 复制表格/文字时会附带一张整体截图，有文字时按文字粘贴
        async handlePaste(event) {
            const data = event.clipboardData;
            if (!data || data.types.includes('text/plain')) return;
            const images = [...data.files].filter(file => file.type.startsWith('image/'));
            if (!images.length) return;
            event.preventDefault();
            for (const file of images) {
                await this.uploadAndInsertImage(file);
            }
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

            // 在光标位置插入唯一占位符
            const placeholder = `![上传中...(${Date.now()})]()`;
            const { selectionStart: start, selectionEnd: end } = this.$refs.input;
            this.replaceRange(start, end, placeholder);

            try {
                const formData = new FormData();
                formData.append('image', file);

                const response = await fetch('/api/upload', {
                    method: 'POST',
                    body: formData
                });

                // 会话过期时会被重定向到登录页，拿到的是 HTML
                if (response.redirected) {
                    throw new Error('登录已过期，请刷新页面重新登录');
                }
                const data = await response.json().catch(() => ({}));
                if (!response.ok || !data.success) {
                    throw new Error(data.error || `服务器返回 ${response.status}`);
                }

                // 文件名里的方括号会破坏 Markdown 图片语法
                const fileName = (file.name || 'image').replace(/\.[^.]+$/, '').replace(/[[\]]/g, '');
                this.replacePlaceholder(placeholder, `![${fileName}](${data.url})`);

            } catch (error) {
                console.error('图片上传失败:', error);
                this.replacePlaceholder(placeholder, '');
                alert('图片上传失败: ' + (error.message || '请重试'));
            } finally {
                this.isUploading = false;
            }
        },

        // 上传期间用户可能继续输入，替换占位符时保持用户的光标位置
        replacePlaceholder(placeholder, text) {
            const textarea = this.$refs.input;
            const index = textarea.value.indexOf(placeholder);
            if (index < 0) return;
            const end = index + placeholder.length;

            // 用户已切到别的输入框（如标题），不抢焦点，直接改值
            if (document.activeElement !== textarea) {
                textarea.setRangeText(text, index, end, 'preserve');
                textarea.dispatchEvent(new Event('input', { bubbles: true }));
                return;
            }
            const shift = pos => pos >= end ? pos + text.length - placeholder.length : Math.min(pos, index + text.length);
            this.replaceRange(index, end, text, shift(textarea.selectionStart), shift(textarea.selectionEnd));
        }
    };
}
