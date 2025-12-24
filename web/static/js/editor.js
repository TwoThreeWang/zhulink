document.addEventListener('alpine:init', () => {
    Alpine.data('markdownEditor', () => ({
        content: '',
        isPreview: false,
        isFullScreen: false,
        isSplitView: false,

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
        }
    }));
});
