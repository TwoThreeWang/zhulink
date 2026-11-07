// 工作量证明（PoW）人机校验，与 internal/services/pow.go 配套。
// 找一个 nonce，使 SHA-256(salt + ":" + nonce) 的前 bits 位全为 0。
// 同一个文件既在页面里运行，也作为 Web Worker 在后台计算，不卡页面。
// 不用 crypto.subtle：它是异步的，逐个哈希太慢，而且只在 HTTPS 下可用。
(function () {
    'use strict';

    var K = [
        0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
        0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
        0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
        0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
        0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
        0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
        0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
        0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2
    ];
    var W = new Int32Array(64);

    function ror(x, n) {
        return (x >>> n) | (x << (32 - n));
    }

    // 返回 SHA-256 结果的前 32 位。msg 只含 ASCII（十六进制 salt、冒号、数字）。
    function sha256Word0(msg) {
        var len = msg.length;
        var blocks = (len + 9 + 63) >> 6;
        var words = new Int32Array(blocks * 16);
        for (var i = 0; i < len; i++) {
            words[i >> 2] |= msg.charCodeAt(i) << (24 - (i & 3) * 8);
        }
        words[len >> 2] |= 0x80 << (24 - (len & 3) * 8);
        words[blocks * 16 - 1] = len * 8;

        var h0 = 0x6a09e667 | 0, h1 = 0xbb67ae85 | 0, h2 = 0x3c6ef372 | 0, h3 = 0xa54ff53a | 0;
        var h4 = 0x510e527f | 0, h5 = 0x9b05688c | 0, h6 = 0x1f83d9ab | 0, h7 = 0x5be0cd19 | 0;

        for (var b = 0; b < blocks; b++) {
            var t;
            for (t = 0; t < 16; t++) W[t] = words[b * 16 + t];
            for (t = 16; t < 64; t++) {
                var w15 = W[t - 15], w2 = W[t - 2];
                var s0 = ror(w15, 7) ^ ror(w15, 18) ^ (w15 >>> 3);
                var s1 = ror(w2, 17) ^ ror(w2, 19) ^ (w2 >>> 10);
                W[t] = (W[t - 16] + s0 + W[t - 7] + s1) | 0;
            }
            var a = h0, c1 = h1, c2 = h2, d = h3, e = h4, f = h5, g = h6, h = h7;
            for (t = 0; t < 64; t++) {
                var S1 = ror(e, 6) ^ ror(e, 11) ^ ror(e, 25);
                var ch = (e & f) ^ (~e & g);
                var t1 = (h + S1 + ch + K[t] + W[t]) | 0;
                var S0 = ror(a, 2) ^ ror(a, 13) ^ ror(a, 22);
                var maj = (a & c1) ^ (a & c2) ^ (c1 & c2);
                var t2 = (S0 + maj) | 0;
                h = g; g = f; f = e; e = (d + t1) | 0;
                d = c2; c2 = c1; c1 = a; a = (t1 + t2) | 0;
            }
            h0 = (h0 + a) | 0; h1 = (h1 + c1) | 0; h2 = (h2 + c2) | 0; h3 = (h3 + d) | 0;
            h4 = (h4 + e) | 0; h5 = (h5 + f) | 0; h6 = (h6 + g) | 0; h7 = (h7 + h) | 0;
        }
        return h0;
    }

    function leadingZeroBits(salt, nonce) {
        return Math.clz32(sha256Word0(salt + ':' + nonce));
    }

    // 从 start 开始找，最多试 count 个；找到返回 nonce，否则返回 -1
    function search(salt, bits, start, count) {
        for (var n = start, end = start + count; n < end; n++) {
            if (leadingZeroBits(salt, n) >= bits) return n;
        }
        return -1;
    }

    // 给 Node 测试用
    if (typeof module !== 'undefined' && module.exports) {
        module.exports = { leadingZeroBits: leadingZeroBits, search: search };
        return;
    }

    // Worker 里：收到挑战就一直算到找到为止
    if (typeof document === 'undefined') {
        self.onmessage = function (ev) {
            var n = search(ev.data.salt, ev.data.bits, 0, Number.MAX_SAFE_INTEGER);
            self.postMessage({ nonce: String(n) });
        };
        return;
    }

    var scriptURL = document.currentScript && document.currentScript.src;
    // 服务端要求挑战签发后至少 2 秒才能提交。脚本运行时挑战已经签发，从现在起等 2.1 秒一定够；
    // 密码管理器自动填充后立刻回车这类情况，就等到这个时刻再自动提交，而不是让用户看到报错
    var readyAt = Date.now() + 2100;

    function attach(form) {
        var salt = form.getAttribute('data-pow-salt');
        var bits = parseInt(form.getAttribute('data-pow-bits'), 10);
        var input = form.querySelector('input[name="pow_nonce"]');
        var button = form.querySelector('[type="submit"]');
        var solved = false;
        var waiting = false;

        function submitWhenReady() {
            setTimeout(function () { form.submit(); }, Math.max(0, readyAt - Date.now()));
        }

        function finish(nonce) {
            input.value = nonce;
            solved = true;
            if (waiting) submitWhenReady();
        }

        // 用户点提交时还没算完或还不到 2 秒：先拦下来，条件满足后自动提交
        form.addEventListener('submit', function (ev) {
            if (solved && Date.now() >= readyAt) return;
            ev.preventDefault();
            if (waiting) return;
            waiting = true;
            if (button) {
                button.disabled = true;
                button.textContent = '正在进行安全校验…';
            }
            if (solved) submitWhenReady();
        });

        try {
            var worker = new Worker(scriptURL);
            worker.onmessage = function (ev) {
                worker.terminate();
                finish(ev.data.nonce);
            };
            worker.postMessage({ salt: salt, bits: bits });
        } catch (err) {
            // 不支持 Worker 时在主线程分批算，每批之间让出线程
            var next = 0;
            (function step() {
                var n = search(salt, bits, next, 20000);
                if (n >= 0) return finish(String(n));
                next += 20000;
                setTimeout(step, 0);
            })();
        }
    }

    var forms = document.querySelectorAll('form[data-pow-salt]');
    for (var i = 0; i < forms.length; i++) attach(forms[i]);
})();
