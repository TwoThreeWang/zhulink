// 生成内联 SVG 图标 sprite：扫描模板和 Go 代码里用到的 #i-<name>，
// 从 lucide 取对应图标，写入 web/templates/components/icons.html（由 base.html 内联一次）。
// 新增图标后运行：make icons
import { readFileSync, writeFileSync, readdirSync, statSync } from 'node:fs';
import { join } from 'node:path';
import * as lucide from 'lucide';

const OUT = 'web/templates/components/icons.html';

function walk(dir, exts, files = []) {
    for (const name of readdirSync(dir)) {
        const path = join(dir, name);
        if (statSync(path).isDirectory()) walk(path, exts, files);
        else if (exts.some(ext => name.endsWith(ext)) && path !== OUT) files.push(path);
    }
    return files;
}

const names = new Set();
for (const file of [...walk('web/templates', ['.html']), ...walk('internal', ['.go'])]) {
    for (const [, name] of readFileSync(file, 'utf8').matchAll(/href="#i-([a-z0-9-]+)"/g)) names.add(name);
}

const pascal = name => name.replace(/(^|-)([a-z0-9])/g, (_, __, c) => c.toUpperCase());
const attrs = obj => Object.entries(obj).map(([k, v]) => ` ${k}="${v}"`).join('');

const symbols = [...names].sort().map(name => {
    const node = lucide.icons[pascal(name)] ?? lucide[pascal(name)];
    if (!node) throw new Error(`lucide 里没有图标: ${name}`);
    const body = node.map(([tag, a]) => `<${tag}${attrs(a)}/>`).join('');
    return `<symbol id="i-${name}" viewBox="0 0 24 24">${body}</symbol>`;
});

writeFileSync(OUT, `{{ define "icon_sprite" }}<svg xmlns="http://www.w3.org/2000/svg" style="display:none">${symbols.join('')}</svg>{{ end }}\n`);
console.log(`${symbols.length} icons -> ${OUT}`);
