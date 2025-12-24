/** @type {import('tailwindcss').Config} */
module.exports = {
    content: ["./web/templates/**/*.html", "./internal/**/*.go"],
    theme: {
        extend: {
            colors: {
                // 米宣纸色 - Rice Paper Background
                paper: '#fdfbf7',
                // 深岩灰 - Deep Stone Ink
                ink: {
                    DEFAULT: '#292524',
                    light: '#57534e',
                },
                // 深苔绿 - Deep Moss Accent
                moss: {
                    DEFAULT: '#15803d',
                    dark: '#14532d',
                },
                // 辅助色
                mist: '#e7e5e4',
            },
            fontFamily: {
                // 统一字体栈 - 优先系统原生字体，兼容多平台
                sans: [
                    '-apple-system',
                    'BlinkMacSystemFont',
                    '"Helvetica Neue"',
                    '"PingFang SC"',
                    '"Microsoft YaHei"',
                    '"Source Han Sans SC"',
                    '"Noto Sans CJK SC"',
                    '"WenQuanYi Micro Hei"',
                    'sans-serif',
                ],
            },
        },
    },
    plugins: [],
}
