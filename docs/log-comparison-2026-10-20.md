# Đối chiếu HTTP trace ngày 20/10/2026
A = account_c2193113e3fd: thành công lúc 10:48:38–10:48:50 (UTC+7). B = account_27e18be2ed54: bị từ chối lúc 10:50:33–10:50:34. Định danh bằng SAPISID trong log đối chiếu hai cookie file hiện tại; các file này có thể đã cập nhật sau lần chạy. Đã đọc 22 body sidecar và kiểm tra độ dài với log. Không gửi lại request lên Google. Giá trị credential/token được thay bằng độ dài và SHA-256 rút gọn.

## Kết luận và giới hạn
B bị từ chối tại ogiZ0b: HTTP 200, data=null, ErrorInfo PUBLIC_ERROR_UNUSUAL_ACTIVITY, mã RPC 7. A trả media rồi tải JPEG thành công. Backend từ chối do hoạt động bất thường; chưa đủ bằng chứng xác định tín hiệu nội bộ nào gây quyết định này. Trace không có score/invalidReason/assessment ID, IP hoặc proxy. Không thể kết luận chắc account bị ban, thiếu credit hay phiên reCAPTCHA là nguyên nhân.

Cả hai cùng trình tự, shape và các field cố định. Khác account cookies, ngôn ngữ, CSRF/session/project, seed/UUID, challenge/token và phiên reCAPTCHA do Google trả về. Tất cả null và vị trí mảng generation trùng nhau. Chênh 42 byte generation body = 2 × (2382 − 2361), vì token xuất hiện hai lần.

Trace chụp req.Header trước c.hc.Do; transport có cookie jar riêng. Thiếu Cookie trong entry reCAPTCHA chỉ chứng minh caller chưa gắn Cookie lúc chụp, không chứng minh wire cuối cùng không có Cookie. Jar có thể lưu/gắn cookie sau đó. Header-Order: và PHeader-Order: là metadata của fhttp, không phải tên HTTP header literal. Trace không ghi wire HTTP/2 hay TLS ClientHello/IP.

## Trình tự và thời gian
| Step | Purpose | A | B |
| --- | --- | --- | --- |
| 1 | credits read | 1792468118506296700-000009; 2026-10-20T10:48:38.506296+07:00; 362 ms; HTTP 200; req/resp 121/184 byte | 1792468233184403100-000003; 2026-10-20T10:50:33.184403+07:00; 355 ms; HTTP 200; req/resp 121/181 byte |
| 2 | release version | 1792468118872689100-000010; 2026-10-20T10:48:38.873240+07:00; 50 ms; HTTP 200; req/resp 0/1731 byte | 1792468233542803300-000004; 2026-10-20T10:50:33.542803+07:00; 156 ms; HTTP 200; req/resp 0/1731 byte |
| 3 | anchor challenge | 1792468118924692400-000011; 2026-10-20T10:48:38.924692+07:00; 85 ms; HTTP 200; req/resp 0/55028 byte | 1792468233699551100-000005; 2026-10-20T10:50:33.699551+07:00; 93 ms; HTTP 200; req/resp 0/55268 byte |
| 4 | reload token | 1792468119011070100-000012; 2026-10-20T10:48:39.011070+07:00; 160 ms; HTTP 200; req/resp 2140/2881 byte | 1792468233794151800-000006; 2026-10-20T10:50:33.794151+07:00; 107 ms; HTTP 200; req/resp 2140/2860 byte |
| 5 | generation | 1792468119172976800-000013; 2026-10-20T10:48:39.173521+07:00; 10528 ms; HTTP 200; req/resp 5539/1328 byte | 1792468233902307700-000007; 2026-10-20T10:50:33.902840+07:00; 344 ms; HTTP 200; req/resp 5497/228 byte |
| 6 | CDN download | 1792468129703369800-000014; 2026-10-20T10:48:49.710060+07:00; 323 ms; HTTP 200; req/resp 0/237794 byte | absent |

Ảnh không có polling: ogiZ0b trả URL inline rồi A tải CDN. Trong hai đoạn cung cấp không có upload ảnh tham chiếu, refresh login, retry CSRF 400 hoặc retry 401. Cả hai dùng HTTP/2.0.

A: khoảng trống kết thúc request → bắt đầu request sau: 1→2: 4.944 ms; 2→3: 1.452 ms; 3→4: 1.378 ms; 4→5: 2.451 ms; 5→6: 8.539 ms. Duration trong log làm tròn tới ms.
B: khoảng trống: 1→2: 3.400 ms; 2→3: 0.748 ms; 3→4: 1.600 ms; 4→5: 1.689 ms.

## Mỗi query key, header, form key và JSON leaf
### Bước 1: nzlxg — đọc credit
Method/path giống nhau: `POST flow.google.com/_/AiSandboxAngularFrontend/data/batchexecute`.
Query:
| Key | A | B | Comparison |
| --- | --- | --- | --- |
| _reqid | `101000` | `101000` | same |
| bl | `boq_labs-ai-sandbox-frontend_20260922.00_p0` | `boq_labs-ai-sandbox-frontend_20260922.00_p0` | same |
| f.sid | `1404839919656137433` | `6903702824746842711` | DIFF |
| hl | `en` | `en` | same |
| rpcids | `nzlxg` | `nzlxg` | same |
| rt | `c` | `c` | same |
| source-path | `/` | `/` | same |

request headers:
| Key | A | B | Comparison |
| --- | --- | --- | --- |
| Accept | `*/*` | `*/*` | same |
| Accept-Language | `vi-VN` | `en-US` | DIFF |
| Authorization | len=67; sha256[:12]=d755ffcd23d4 | len=67; sha256[:12]=d090ef97631c | DIFF |
| Content-Type | `application/x-www-form-urlencoded;charset=UTF-8` | `application/x-www-form-urlencoded;charset=UTF-8` | same |
| Cookie | len=1157; sha256[:12]=c9e49966187a | len=1173; sha256[:12]=dfd9cad22439 | DIFF |
| Header-Order: | `host`; `sec-ch-ua`; `sec-ch-ua-mobile`; `sec-ch-ua-platform`; `sec-ch-ua-arch`; `sec-ch-ua-bitness`; `sec-ch-ua-full-version`; `user-agent`; `accept`; `accept-encoding`; `accept-language`; `authorization`; `content-type`; `origin`; `referer`; `cookie`; `x-goog-api-key`; `priority` | `host`; `sec-ch-ua`; `sec-ch-ua-mobile`; `sec-ch-ua-platform`; `sec-ch-ua-arch`; `sec-ch-ua-bitness`; `sec-ch-ua-full-version`; `user-agent`; `accept`; `accept-encoding`; `accept-language`; `authorization`; `content-type`; `origin`; `referer`; `cookie`; `x-goog-api-key`; `priority` | same |
| Origin | `https://flow.google.com` | `https://flow.google.com` | same |
| PHeader-Order: | `:method`; `:authority`; `:scheme`; `:path` | `:method`; `:authority`; `:scheme`; `:path` | same |
| Priority | `u=1, i` | `u=1, i` | same |
| Referer | `https://flow.google.com/` | `https://flow.google.com/` | same |
| Sec-Ch-Ua | `"Google Chrome";v="155", "Chromium";v="155", "Not(A:Brand";v="24"` | `"Google Chrome";v="155", "Chromium";v="155", "Not(A:Brand";v="24"` | same |
| Sec-Ch-Ua-Arch | `"x86"` | `"x86"` | same |
| Sec-Ch-Ua-Bitness | `"64"` | `"64"` | same |
| Sec-Ch-Ua-Full-Version | `"152.0.0.0"` | `"152.0.0.0"` | same |
| Sec-Ch-Ua-Mobile | `?0` | `?0` | same |
| Sec-Ch-Ua-Platform | `"Windows"` | `"Windows"` | same |
| Sec-Fetch-Dest | `empty` | `empty` | same |
| Sec-Fetch-Mode | `cors` | `cors` | same |
| Sec-Fetch-Site | `same-origin` | `same-origin` | same |
| User-Agent | `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/155.0.0.0 Safari/537.36` | `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/155.0.0.0 Safari/537.36` | same |
| X-Same-Domain | `1` | `1` | same |

response headers:
| Key | A | B | Comparison |
| --- | --- | --- | --- |
| Accept-Ch | `Sec-CH-UA-Arch, Sec-CH-UA-Bitness, Sec-CH-UA-Full-Version, Sec-CH-UA-Full-Version-List, Sec-CH-UA-Model, Sec-CH-UA-WoW64, Sec-CH-UA-Form-Factors, Sec-CH-UA-Platform, Sec-CH-UA-Platform-Version` | `Sec-CH-UA-Arch, Sec-CH-UA-Bitness, Sec-CH-UA-Full-Version, Sec-CH-UA-Full-Version-List, Sec-CH-UA-Model, Sec-CH-UA-WoW64, Sec-CH-UA-Form-Factors, Sec-CH-UA-Platform, Sec-CH-UA-Platform-Version` | same |
| Alt-Svc | `h3=":443"; ma=2592000,h3-29=":443"; ma=2592000` | `h3=":443"; ma=2592000,h3-29=":443"; ma=2592000` | same |
| Cache-Control | `no-cache, no-store, max-age=0, must-revalidate` | `no-cache, no-store, max-age=0, must-revalidate` | same |
| Content-Disposition | `attachment; filename="response.bin"; filename*=UTF-8''response.bin` | `attachment; filename="response.bin"; filename*=UTF-8''response.bin` | same |
| Content-Encoding | `gzip` | `gzip` | same |
| Content-Security-Policy | `require-trusted-types-for 'script';report-uri /_/AiSandboxAngularFrontend/cspreport` | `require-trusted-types-for 'script';report-uri /_/AiSandboxAngularFrontend/cspreport` | same |
| Content-Type | `application/json; charset=utf-8` | `application/json; charset=utf-8` | same |
| Cross-Origin-Opener-Policy | `same-origin-allow-popups` | `same-origin-allow-popups` | same |
| Cross-Origin-Resource-Policy | `same-site` | `same-site` | same |
| Date | `Tue, 20 Oct 2026 03:48:38 GMT` | `Tue, 20 Oct 2026 03:50:33 GMT` | DIFF |
| Expires | `Mon, 01 Jan 1990 00:00:00 GMT` | `Mon, 01 Jan 1990 00:00:00 GMT` | same |
| Permissions-Policy | `ch-ua-arch=*, ch-ua-bitness=*, ch-ua-full-version=*, ch-ua-full-version-list=*, ch-ua-model=*, ch-ua-wow64=*, ch-ua-form-factors=*, ch-ua-platform=*, ch-ua-platform-version=*` | `ch-ua-arch=*, ch-ua-bitness=*, ch-ua-full-version=*, ch-ua-full-version-list=*, ch-ua-model=*, ch-ua-wow64=*, ch-ua-form-factors=*, ch-ua-platform=*, ch-ua-platform-version=*` | same |
| Pragma | `no-cache` | `no-cache` | same |
| Server | `ESF` | `ESF` | same |
| Set-Cookie | `SIDCC=<value sha256:a06e59d17879>; expires=<value sha256:7199efe5578f>; path=<value sha256:8a5edab28263>; domain=<value sha256:5d59719991ce>; priority=high`; `__Secure-1PSIDCC=<value sha256:9650dccc4ff9>; expires=<value sha256:7199efe5578f>; path=<value sha256:8a5edab28263>; domain=<value sha256:5d59719991ce>; Secure; HttpOnly; priority=high`; `__Secure-3PSIDCC=<value sha256:3adc2447f2db>; expires=<value sha256:7199efe5578f>; path=<value sha256:8a5edab28263>; domain=<value sha256:5d59719991ce>; Secure; HttpOnly; priority=<value sha256:6ef7c9b15ecd>; SameSite=none` | `SIDCC=<value sha256:bcd1d89060b0>; expires=<value sha256:d685f698cfe8>; path=<value sha256:8a5edab28263>; domain=<value sha256:5d59719991ce>; priority=high`; `__Secure-1PSIDCC=<value sha256:914b08b079ee>; expires=<value sha256:d685f698cfe8>; path=<value sha256:8a5edab28263>; domain=<value sha256:5d59719991ce>; Secure; HttpOnly; priority=high`; `__Secure-3PSIDCC=<value sha256:4a178e436867>; expires=<value sha256:d685f698cfe8>; path=<value sha256:8a5edab28263>; domain=<value sha256:5d59719991ce>; Secure; HttpOnly; priority=<value sha256:6ef7c9b15ecd>; SameSite=none` | DIFF |
| Vary | `Sec-Fetch-Dest, Sec-Fetch-Mode, Sec-Fetch-Site` | `Sec-Fetch-Dest, Sec-Fetch-Mode, Sec-Fetch-Site` | same |
| X-Content-Type-Options | `nosniff` | `nosniff` | same |
| X-Frame-Options | `SAMEORIGIN` | `SAMEORIGIN` | same |
| X-Xss-Protection | `0` | `0` | same |

Set-Cookie theo tên (giá trị thay bằng hash, giữ attributes):
| Cookie name | A | B |
| --- | --- | --- |
| SIDCC | value sha=a06e59d17879;  expires=Wed, 20-Oct-2027 03:48:38 GMT; path=/; domain=.google.com; priority=high | value sha=bcd1d89060b0;  expires=Wed, 20-Oct-2027 03:50:33 GMT; path=/; domain=.google.com; priority=high |
| __Secure-1PSIDCC | value sha=9650dccc4ff9;  expires=Wed, 20-Oct-2027 03:48:38 GMT; path=/; domain=.google.com; Secure; HttpOnly; priority=high | value sha=914b08b079ee;  expires=Wed, 20-Oct-2027 03:50:33 GMT; path=/; domain=.google.com; Secure; HttpOnly; priority=high |
| __Secure-3PSIDCC | value sha=3adc2447f2db;  expires=Wed, 20-Oct-2027 03:48:38 GMT; path=/; domain=.google.com; Secure; HttpOnly; priority=high; SameSite=none | value sha=4a178e436867;  expires=Wed, 20-Oct-2027 03:50:33 GMT; path=/; domain=.google.com; Secure; HttpOnly; priority=high; SameSite=none |

Form keys:
| Key | A | B | Comparison |
| --- | --- | --- | --- |
| at | len=42; sha256[:12]=112afd3e4f17 | len=42; sha256[:12]=fec8eab2b30c | DIFF |
| f.req | `<JSON expanded below>` | `<JSON expanded below>` | same |

Envelope: `[[[rpcid, JSON.stringify(argument), null, "generic"]]]`; rpcid khớp query. Form chỉ có `at`, `f.req`.
Argument sau giải mã từng tầng JSON (zero-based, mọi scalar/null):
| Path | A | B | Comparison |
| --- | --- | --- | --- |
| $ |  |  | same |

Response sau bỏ XSSI/length framing và giải mã payload string:
| Path | A | B | Comparison |
| --- | --- | --- | --- |
| $[0][0] | `wrb.fr` | `wrb.fr` | same |
| $[0][1] | `nzlxg` | `nzlxg` | same |
| $[0][2][0] | 888 | 50 | DIFF |
| $[0][2][1] | 1 | 3 | DIFF |
| $[0][2][2] | 2 | 8 | DIFF |
| $[0][2][3] | 2 | 1 | DIFF |
| $[0][2][4] | null | null | same |
| $[0][2][5] | 888 | 50 | DIFF |
| $[0][2][6][0] | 1793871876 | 1792554633 | DIFF |
| $[0][2][6][1] | 115491000 | 296744000 | DIFF |
| $[0][3] | null | null | same |
| $[0][4] | null | null | same |
| $[0][5] | null | null | same |
| $[0][6] | `generic` | `generic` | same |
| $[1][0] | `di` | `di` | same |
| $[1][1] | 319 | 316 | DIFF |
| $[2][0] | `af.httprm` | `af.httprm` | same |
| $[2][1] | 319 | 315 | DIFF |
| $[2][2] | `-2198078624620869815` | `2602864786332590103` | DIFF |
| $[2][3] | 25 | 19 | DIFF |
| $[3][0] | `e` | `e` | same |
| $[3][1] | 4 | 4 | same |
| $[3][2] | null | null | same |
| $[3][3] | null | null | same |
| $[3][4] | 184 | 181 | DIFF |

### Bước 2: enterprise.js — lấy release version
Method/path giống nhau: `GET www.google.com/recaptcha/enterprise.js`.
Query:
| Key | A | B | Comparison |
| --- | --- | --- | --- |
| render | `6LdsFiUsAAAAAIjVDZcuLhaHiDn5nnHVXVRQGeMV` | `6LdsFiUsAAAAAIjVDZcuLhaHiDn5nnHVXVRQGeMV` | same |

request headers:
| Key | A | B | Comparison |
| --- | --- | --- | --- |
| Accept | `*/*` | `*/*` | same |
| Accept-Language | `en-US,en;q=0.9` | `en-US,en;q=0.9` | same |
| Header-Order: | `host`; `sec-ch-ua`; `sec-ch-ua-mobile`; `sec-ch-ua-platform`; `sec-ch-ua-arch`; `sec-ch-ua-bitness`; `sec-ch-ua-full-version`; `user-agent`; `accept`; `accept-encoding`; `accept-language`; `authorization`; `content-type`; `origin`; `referer`; `cookie`; `x-goog-api-key`; `priority` | `host`; `sec-ch-ua`; `sec-ch-ua-mobile`; `sec-ch-ua-platform`; `sec-ch-ua-arch`; `sec-ch-ua-bitness`; `sec-ch-ua-full-version`; `user-agent`; `accept`; `accept-encoding`; `accept-language`; `authorization`; `content-type`; `origin`; `referer`; `cookie`; `x-goog-api-key`; `priority` | same |
| PHeader-Order: | `:method`; `:authority`; `:scheme`; `:path` | `:method`; `:authority`; `:scheme`; `:path` | same |
| Priority | `u=1, i` | `u=1, i` | same |
| Referer | `https://flow.google.com/` | `https://flow.google.com/` | same |
| Sec-Ch-Ua | `"Not(A:Brand";v="99", "Google Chrome";v="152", "Chromium";v="152"` | `"Not(A:Brand";v="99", "Google Chrome";v="152", "Chromium";v="152"` | same |
| Sec-Ch-Ua-Arch | `"x86"` | `"x86"` | same |
| Sec-Ch-Ua-Bitness | `"64"` | `"64"` | same |
| Sec-Ch-Ua-Full-Version | `"152.0.0.0"` | `"152.0.0.0"` | same |
| Sec-Ch-Ua-Mobile | `?0` | `?0` | same |
| Sec-Ch-Ua-Platform | `"Windows"` | `"Windows"` | same |
| User-Agent | `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/155.0.0.0 Safari/537.36` | `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/155.0.0.0 Safari/537.36` | same |

response headers:
| Key | A | B | Comparison |
| --- | --- | --- | --- |
| Alt-Svc | `h3=":443"; ma=2592000,h3-29=":443"; ma=2592000` | `h3=":443"; ma=2592000,h3-29=":443"; ma=2592000` | same |
| Cache-Control | `private, max-age=300` | `private, max-age=300` | same |
| Content-Encoding | `gzip` | `gzip` | same |
| Content-Type | `text/javascript; charset=utf-8` | `text/javascript; charset=utf-8` | same |
| Cross-Origin-Opener-Policy-Report-Only | `same-origin; report-to="coop_38fac9d5b82543fc4729580d18ff2d3d"` | `same-origin; report-to="coop_38fac9d5b82543fc4729580d18ff2d3d"` | same |
| Cross-Origin-Resource-Policy | `cross-origin` | `cross-origin` | same |
| Date | `Tue, 20 Oct 2026 03:48:38 GMT` | `Tue, 20 Oct 2026 03:50:33 GMT` | DIFF |
| Expires | `Tue, 20 Oct 2026 03:48:38 GMT` | `Tue, 20 Oct 2026 03:50:33 GMT` | DIFF |
| Report-To | `{"group":"coop_38fac9d5b82543fc4729580d18ff2d3d","max_age":2592000,"endpoints":[{"url":"https://csp.withgoogle.com/csp/report-to/38fac9d5b82543fc4729580d18ff2d3d"}]}` | `{"group":"coop_38fac9d5b82543fc4729580d18ff2d3d","max_age":2592000,"endpoints":[{"url":"https://csp.withgoogle.com/csp/report-to/38fac9d5b82543fc4729580d18ff2d3d"}]}` | same |
| Server | `ESF` | `ESF` | same |
| Set-Cookie | `SIDCC=<value sha256:b0055e6950e4>; expires=<value sha256:00f6ebb2f15c>; path=<value sha256:8a5edab28263>; domain=<value sha256:5d59719991ce>; priority=high`; `__Secure-1PSIDCC=<value sha256:b0055e6950e4>; expires=<value sha256:00f6ebb2f15c>; path=<value sha256:8a5edab28263>; domain=<value sha256:5d59719991ce>; Secure; HttpOnly; priority=high`; `__Secure-3PSIDCC=<value sha256:b0055e6950e4>; expires=<value sha256:00f6ebb2f15c>; path=<value sha256:8a5edab28263>; domain=<value sha256:5d59719991ce>; Secure; HttpOnly; priority=<value sha256:6ef7c9b15ecd>; SameSite=none` | `SIDCC=<value sha256:b0055e6950e4>; expires=<value sha256:00f6ebb2f15c>; path=<value sha256:8a5edab28263>; domain=<value sha256:5d59719991ce>; priority=high`; `__Secure-1PSIDCC=<value sha256:b0055e6950e4>; expires=<value sha256:00f6ebb2f15c>; path=<value sha256:8a5edab28263>; domain=<value sha256:5d59719991ce>; Secure; HttpOnly; priority=high`; `__Secure-3PSIDCC=<value sha256:b0055e6950e4>; expires=<value sha256:00f6ebb2f15c>; path=<value sha256:8a5edab28263>; domain=<value sha256:5d59719991ce>; Secure; HttpOnly; priority=<value sha256:6ef7c9b15ecd>; SameSite=none` | same |
| X-Content-Type-Options | `nosniff` | `nosniff` | same |
| X-Frame-Options | `SAMEORIGIN` | `SAMEORIGIN` | same |
| X-Xss-Protection | `0` | `0` | same |

Set-Cookie theo tên (giá trị thay bằng hash, giữ attributes):
| Cookie name | A | B |
| --- | --- | --- |
| SIDCC | value sha=b0055e6950e4;  expires=Mon, 01-Jan-1990 00:00:00 GMT; path=/; domain=.google.com; priority=high | value sha=b0055e6950e4;  expires=Mon, 01-Jan-1990 00:00:00 GMT; path=/; domain=.google.com; priority=high |
| __Secure-1PSIDCC | value sha=b0055e6950e4;  expires=Mon, 01-Jan-1990 00:00:00 GMT; path=/; domain=.google.com; Secure; HttpOnly; priority=high | value sha=b0055e6950e4;  expires=Mon, 01-Jan-1990 00:00:00 GMT; path=/; domain=.google.com; Secure; HttpOnly; priority=high |
| __Secure-3PSIDCC | value sha=b0055e6950e4;  expires=Mon, 01-Jan-1990 00:00:00 GMT; path=/; domain=.google.com; Secure; HttpOnly; priority=high; SameSite=none | value sha=b0055e6950e4;  expires=Mon, 01-Jan-1990 00:00:00 GMT; path=/; domain=.google.com; Secure; HttpOnly; priority=high; SameSite=none |

### Bước 3: anchor — lấy challenge
Method/path giống nhau: `GET www.google.com/recaptcha/enterprise/anchor`.
Query:
| Key | A | B | Comparison |
| --- | --- | --- | --- |
| anchor-ms | `20000` | `20000` | same |
| ar | `1` | `1` | same |
| cb | `sngzqswl3veg` | `sz132pwv9x18` | DIFF |
| co | `aHR0cHM6Ly9mbG93Lmdvb2dsZS5jb206NDQz.` | `aHR0cHM6Ly9mbG93Lmdvb2dsZS5jb206NDQz.` | same |
| execute-ms | `30000` | `30000` | same |
| hl | `en` | `en` | same |
| k | `6LdsFiUsAAAAAIjVDZcuLhaHiDn5nnHVXVRQGeMV` | `6LdsFiUsAAAAAIjVDZcuLhaHiDn5nnHVXVRQGeMV` | same |
| size | `invisible` | `invisible` | same |
| v | `4EDjxXf4knsx6UBM1L2V6cv8` | `NhfdZHzre16wrc2m6BxEdu7J` | DIFF |

request headers:
| Key | A | B | Comparison |
| --- | --- | --- | --- |
| Accept | `text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8` | `text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8` | same |
| Accept-Language | `en-US,en;q=0.9` | `en-US,en;q=0.9` | same |
| Header-Order: | `host`; `sec-ch-ua`; `sec-ch-ua-mobile`; `sec-ch-ua-platform`; `sec-ch-ua-arch`; `sec-ch-ua-bitness`; `sec-ch-ua-full-version`; `user-agent`; `accept`; `accept-encoding`; `accept-language`; `authorization`; `content-type`; `origin`; `referer`; `cookie`; `x-goog-api-key`; `priority` | `host`; `sec-ch-ua`; `sec-ch-ua-mobile`; `sec-ch-ua-platform`; `sec-ch-ua-arch`; `sec-ch-ua-bitness`; `sec-ch-ua-full-version`; `user-agent`; `accept`; `accept-encoding`; `accept-language`; `authorization`; `content-type`; `origin`; `referer`; `cookie`; `x-goog-api-key`; `priority` | same |
| PHeader-Order: | `:method`; `:authority`; `:scheme`; `:path` | `:method`; `:authority`; `:scheme`; `:path` | same |
| Priority | `u=1, i` | `u=1, i` | same |
| Referer | `https://flow.google.com/` | `https://flow.google.com/` | same |
| Sec-Ch-Ua | `"Not(A:Brand";v="99", "Google Chrome";v="152", "Chromium";v="152"` | `"Not(A:Brand";v="99", "Google Chrome";v="152", "Chromium";v="152"` | same |
| Sec-Ch-Ua-Arch | `"x86"` | `"x86"` | same |
| Sec-Ch-Ua-Bitness | `"64"` | `"64"` | same |
| Sec-Ch-Ua-Full-Version | `"152.0.0.0"` | `"152.0.0.0"` | same |
| Sec-Ch-Ua-Mobile | `?0` | `?0` | same |
| Sec-Ch-Ua-Platform | `"Windows"` | `"Windows"` | same |
| Sec-Fetch-Dest | `iframe` | `iframe` | same |
| Sec-Fetch-Mode | `navigate` | `navigate` | same |
| Sec-Fetch-Site | `cross-site` | `cross-site` | same |
| User-Agent | `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/155.0.0.0 Safari/537.36` | `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/155.0.0.0 Safari/537.36` | same |

response headers:
| Key | A | B | Comparison |
| --- | --- | --- | --- |
| Alt-Svc | `h3=":443"; ma=2592000,h3-29=":443"; ma=2592000` | `h3=":443"; ma=2592000,h3-29=":443"; ma=2592000` | same |
| Cache-Control | `no-cache, no-store, max-age=0, must-revalidate` | `no-cache, no-store, max-age=0, must-revalidate` | same |
| Content-Encoding | `gzip` | `gzip` | same |
| Content-Security-Policy | `script-src 'report-sample' 'nonce-nfC39sjPomowvThTe5dpUQ' 'unsafe-inline' 'strict-dynamic' https: http: 'unsafe-eval';object-src 'none';base-uri 'self';report-uri https://csp.withgoogle.com/csp/recaptcha/1` | `script-src 'report-sample' 'nonce-1Qhi-XmF-sMXyRr1P91_AA' 'unsafe-inline' 'strict-dynamic' https: http: 'unsafe-eval';object-src 'none';base-uri 'self';report-uri https://csp.withgoogle.com/csp/recaptcha/1` | DIFF |
| Content-Type | `text/html; charset=utf-8` | `text/html; charset=utf-8` | same |
| Cross-Origin-Embedder-Policy | `require-corp` | `require-corp` | same |
| Cross-Origin-Opener-Policy-Report-Only | `same-origin; report-to="coop_38fac9d5b82543fc4729580d18ff2d3d"` | `same-origin; report-to="coop_38fac9d5b82543fc4729580d18ff2d3d"` | same |
| Cross-Origin-Resource-Policy | `cross-origin` | `cross-origin` | same |
| Date | `Tue, 20 Oct 2026 03:48:38 GMT` | `Tue, 20 Oct 2026 03:50:33 GMT` | DIFF |
| Expires | `Mon, 01 Jan 1990 00:00:00 GMT` | `Mon, 01 Jan 1990 00:00:00 GMT` | same |
| Pragma | `no-cache` | `no-cache` | same |
| Report-To | `{"group":"recaptcha","max_age":2592000,"endpoints":[{"url":"https://csp.withgoogle.com/csp/report-to/recaptcha"}]}`; `{"group":"coop_38fac9d5b82543fc4729580d18ff2d3d","max_age":2592000,"endpoints":[{"url":"https://csp.withgoogle.com/csp/report-to/38fac9d5b82543fc4729580d18ff2d3d"}]}` | `{"group":"recaptcha","max_age":2592000,"endpoints":[{"url":"https://csp.withgoogle.com/csp/report-to/recaptcha"}]}`; `{"group":"coop_38fac9d5b82543fc4729580d18ff2d3d","max_age":2592000,"endpoints":[{"url":"https://csp.withgoogle.com/csp/report-to/38fac9d5b82543fc4729580d18ff2d3d"}]}` | same |
| Server | `ESF` | `ESF` | same |
| X-Content-Type-Options | `nosniff` | `nosniff` | same |
| X-Xss-Protection | `0` | `0` | same |

### Bước 4: reload — đổi challenge lấy token
Method/path giống nhau: `POST www.google.com/recaptcha/enterprise/reload`.
Query:
| Key | A | B | Comparison |
| --- | --- | --- | --- |
| k | `6LdsFiUsAAAAAIjVDZcuLhaHiDn5nnHVXVRQGeMV` | `6LdsFiUsAAAAAIjVDZcuLhaHiDn5nnHVXVRQGeMV` | same |

request headers:
| Key | A | B | Comparison |
| --- | --- | --- | --- |
| Accept | `*/*` | `*/*` | same |
| Accept-Language | `en-US,en;q=0.9` | `en-US,en;q=0.9` | same |
| Content-Type | `application/x-www-form-urlencoded` | `application/x-www-form-urlencoded` | same |
| Header-Order: | `host`; `sec-ch-ua`; `sec-ch-ua-mobile`; `sec-ch-ua-platform`; `sec-ch-ua-arch`; `sec-ch-ua-bitness`; `sec-ch-ua-full-version`; `user-agent`; `accept`; `accept-encoding`; `accept-language`; `authorization`; `content-type`; `origin`; `referer`; `cookie`; `x-goog-api-key`; `priority` | `host`; `sec-ch-ua`; `sec-ch-ua-mobile`; `sec-ch-ua-platform`; `sec-ch-ua-arch`; `sec-ch-ua-bitness`; `sec-ch-ua-full-version`; `user-agent`; `accept`; `accept-encoding`; `accept-language`; `authorization`; `content-type`; `origin`; `referer`; `cookie`; `x-goog-api-key`; `priority` | same |
| Origin | `https://www.google.com` | `https://www.google.com` | same |
| PHeader-Order: | `:method`; `:authority`; `:scheme`; `:path` | `:method`; `:authority`; `:scheme`; `:path` | same |
| Priority | `u=1, i` | `u=1, i` | same |
| Referer | `https://www.google.com/recaptcha/enterprise/anchor?anchor-ms=20000&ar=1&cb=sngzqswl3veg&co=aHR0cHM6Ly9mbG93Lmdvb2dsZS5jb206NDQz.&execute-ms=30000&hl=en&k=6LdsFiUsAAAAAIjVDZcuLhaHiDn5nnHVXVRQGeMV&size=invisible&v=4EDjxXf4knsx6UBM1L2V6cv8` | `https://www.google.com/recaptcha/enterprise/anchor?anchor-ms=20000&ar=1&cb=sz132pwv9x18&co=aHR0cHM6Ly9mbG93Lmdvb2dsZS5jb206NDQz.&execute-ms=30000&hl=en&k=6LdsFiUsAAAAAIjVDZcuLhaHiDn5nnHVXVRQGeMV&size=invisible&v=NhfdZHzre16wrc2m6BxEdu7J` | DIFF |
| Sec-Ch-Ua | `"Not(A:Brand";v="99", "Google Chrome";v="152", "Chromium";v="152"` | `"Not(A:Brand";v="99", "Google Chrome";v="152", "Chromium";v="152"` | same |
| Sec-Ch-Ua-Arch | `"x86"` | `"x86"` | same |
| Sec-Ch-Ua-Bitness | `"64"` | `"64"` | same |
| Sec-Ch-Ua-Full-Version | `"152.0.0.0"` | `"152.0.0.0"` | same |
| Sec-Ch-Ua-Mobile | `?0` | `?0` | same |
| Sec-Ch-Ua-Platform | `"Windows"` | `"Windows"` | same |
| Sec-Fetch-Dest | `empty` | `empty` | same |
| Sec-Fetch-Mode | `cors` | `cors` | same |
| Sec-Fetch-Site | `same-origin` | `same-origin` | same |
| User-Agent | `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/155.0.0.0 Safari/537.36` | `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/155.0.0.0 Safari/537.36` | same |

response headers:
| Key | A | B | Comparison |
| --- | --- | --- | --- |
| Alt-Svc | `h3=":443"; ma=2592000,h3-29=":443"; ma=2592000` | `h3=":443"; ma=2592000,h3-29=":443"; ma=2592000` | same |
| Cache-Control | `private` | `private` | same |
| Content-Encoding | `gzip` | `gzip` | same |
| Content-Type | `application/json; charset=utf-8` | `application/json; charset=utf-8` | same |
| Cross-Origin-Opener-Policy-Report-Only | `same-origin; report-to="coop_38fac9d5b82543fc4729580d18ff2d3d"` | `same-origin; report-to="coop_38fac9d5b82543fc4729580d18ff2d3d"` | same |
| Cross-Origin-Resource-Policy | `same-site` | `same-site` | same |
| Date | `Tue, 20 Oct 2026 03:48:39 GMT` | `Tue, 20 Oct 2026 03:50:33 GMT` | DIFF |
| Expires | `Tue, 20 Oct 2026 03:48:39 GMT` | `Tue, 20 Oct 2026 03:50:33 GMT` | DIFF |
| Report-To | `{"group":"coop_38fac9d5b82543fc4729580d18ff2d3d","max_age":2592000,"endpoints":[{"url":"https://csp.withgoogle.com/csp/report-to/38fac9d5b82543fc4729580d18ff2d3d"}]}` | `{"group":"coop_38fac9d5b82543fc4729580d18ff2d3d","max_age":2592000,"endpoints":[{"url":"https://csp.withgoogle.com/csp/report-to/38fac9d5b82543fc4729580d18ff2d3d"}]}` | same |
| Server | `ESF` | `ESF` | same |
| Set-Cookie | `_GRECAPTCHA=<value sha256:c59c34cbfd8a>; Expires=<value sha256:9f4ad40edb4e>; Path=<value sha256:90f126796fc5>; Secure; HttpOnly; Priority=<value sha256:b43e72eed048>; SameSite=none` | `_GRECAPTCHA=<value sha256:c80c72e43990>; Expires=<value sha256:bf4c447c0f68>; Path=<value sha256:90f126796fc5>; Secure; HttpOnly; Priority=<value sha256:b43e72eed048>; SameSite=none` | DIFF |
| Vary | `Sec-Fetch-Dest, Sec-Fetch-Mode, Sec-Fetch-Site` | `Sec-Fetch-Dest, Sec-Fetch-Mode, Sec-Fetch-Site` | same |
| X-Content-Type-Options | `nosniff` | `nosniff` | same |
| X-Frame-Options | `SAMEORIGIN` | `SAMEORIGIN` | same |
| X-Xss-Protection | `0` | `0` | same |

Set-Cookie theo tên (giá trị thay bằng hash, giữ attributes):
| Cookie name | A | B |
| --- | --- | --- |
| _GRECAPTCHA | value sha=c59c34cbfd8a;  Expires=Sun, 18-Apr-2027 03:48:39 GMT; Path=/recaptcha; Secure; HttpOnly; Priority=HIGH; SameSite=none | value sha=c80c72e43990;  Expires=Sun, 18-Apr-2027 03:50:33 GMT; Path=/recaptcha; Secure; HttpOnly; Priority=HIGH; SameSite=none |

Form keys:
| Key | A | B | Comparison |
| --- | --- | --- | --- |
| c | len=1977; sha256[:12]=e9965aab6818 | len=1977; sha256[:12]=fec240cb3cd9 | DIFF |
| co | `aHR0cHM6Ly9mbG93Lmdvb2dsZS5jb206NDQz.` | `aHR0cHM6Ly9mbG93Lmdvb2dsZS5jb206NDQz.` | same |
| hl | `en` | `en` | same |
| k | `6LdsFiUsAAAAAIjVDZcuLhaHiDn5nnHVXVRQGeMV` | `6LdsFiUsAAAAAIjVDZcuLhaHiDn5nnHVXVRQGeMV` | same |
| reason | `q` | `q` | same |
| sa | `IMAGE_GENERATION` | `IMAGE_GENERATION` | same |
| size | `invisible` | `invisible` | same |
| v | `4EDjxXf4knsx6UBM1L2V6cv8` | `NhfdZHzre16wrc2m6BxEdu7J` | DIFF |

Reload response từng slot; chưa có schema xác minh ý nghĩa opaque field:
| Path | A | B | Comparison |
| --- | --- | --- | --- |
| $[0] | `rresp` | `rresp` | same |
| $[10] | null | null | same |
| $[11] | null | null | same |
| $[12] | len=102; sha256[:12]=5ba15652e9fb | len=102; sha256[:12]=d8bbddd5fa28 | DIFF |
| $[13] | null | null | same |
| $[14] | len=68; sha256[:12]=56406c89d818 | len=68; sha256[:12]=111dbfdfd193 | DIFF |
| $[15] | len=76; sha256[:12]=b60b81b583a8 | len=76; sha256[:12]=5c85057a50f6 | DIFF |
| $[16] | 0 | 0 | same |
| $[1] | len=2382; sha256[:12]=065bb1f3aade | len=2361; sha256[:12]=667daecdb342 | DIFF |
| $[2] | null | null | same |
| $[3] | 120 | 120 | same |
| $[4] | null | null | same |
| $[5] | null | null | same |
| $[6] | null | null | same |
| $[7] | null | null | same |
| $[8] | <opaque len=173 sha256[:12]=5644adfb71d0> | <opaque len=173 sha256[:12]=cd40e0359550> | DIFF |
| $[9] | null | null | same |

### Bước 5: ogiZ0b — gửi generation
Method/path giống nhau: `POST flow.google.com/_/AiSandboxAngularFrontend/data/batchexecute`.
Query:
| Key | A | B | Comparison |
| --- | --- | --- | --- |
| _reqid | `101000` | `101000` | same |
| bl | `boq_labs-ai-sandbox-frontend_20260922.00_p0` | `boq_labs-ai-sandbox-frontend_20260922.00_p0` | same |
| f.sid | `1404839919656137433` | `6903702824746842711` | DIFF |
| hl | `en` | `en` | same |
| rpcids | `ogiZ0b` | `ogiZ0b` | same |
| rt | `c` | `c` | same |
| source-path | `/project/78e84224-51c6-44e6-ab3c-1033351153bf` | `/project/7f13ac17-918e-4c58-a170-cc2356e3677e` | DIFF |

request headers:
| Key | A | B | Comparison |
| --- | --- | --- | --- |
| Accept | `*/*` | `*/*` | same |
| Accept-Language | `vi-VN` | `en-US` | DIFF |
| Authorization | len=67; sha256[:12]=73455198ce3c | len=67; sha256[:12]=d090ef97631c | DIFF |
| Content-Type | `application/x-www-form-urlencoded;charset=UTF-8` | `application/x-www-form-urlencoded;charset=UTF-8` | same |
| Cookie | len=1157; sha256[:12]=c9e49966187a | len=1173; sha256[:12]=dfd9cad22439 | DIFF |
| Header-Order: | `host`; `sec-ch-ua`; `sec-ch-ua-mobile`; `sec-ch-ua-platform`; `sec-ch-ua-arch`; `sec-ch-ua-bitness`; `sec-ch-ua-full-version`; `user-agent`; `accept`; `accept-encoding`; `accept-language`; `authorization`; `content-type`; `origin`; `referer`; `cookie`; `x-goog-api-key`; `priority` | `host`; `sec-ch-ua`; `sec-ch-ua-mobile`; `sec-ch-ua-platform`; `sec-ch-ua-arch`; `sec-ch-ua-bitness`; `sec-ch-ua-full-version`; `user-agent`; `accept`; `accept-encoding`; `accept-language`; `authorization`; `content-type`; `origin`; `referer`; `cookie`; `x-goog-api-key`; `priority` | same |
| Origin | `https://flow.google.com` | `https://flow.google.com` | same |
| PHeader-Order: | `:method`; `:authority`; `:scheme`; `:path` | `:method`; `:authority`; `:scheme`; `:path` | same |
| Priority | `u=1, i` | `u=1, i` | same |
| Referer | `https://flow.google.com/` | `https://flow.google.com/` | same |
| Sec-Ch-Ua | `"Google Chrome";v="155", "Chromium";v="155", "Not(A:Brand";v="24"` | `"Google Chrome";v="155", "Chromium";v="155", "Not(A:Brand";v="24"` | same |
| Sec-Ch-Ua-Arch | `"x86"` | `"x86"` | same |
| Sec-Ch-Ua-Bitness | `"64"` | `"64"` | same |
| Sec-Ch-Ua-Full-Version | `"152.0.0.0"` | `"152.0.0.0"` | same |
| Sec-Ch-Ua-Mobile | `?0` | `?0` | same |
| Sec-Ch-Ua-Platform | `"Windows"` | `"Windows"` | same |
| Sec-Fetch-Dest | `empty` | `empty` | same |
| Sec-Fetch-Mode | `cors` | `cors` | same |
| Sec-Fetch-Site | `same-origin` | `same-origin` | same |
| User-Agent | `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/155.0.0.0 Safari/537.36` | `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/155.0.0.0 Safari/537.36` | same |
| X-Same-Domain | `1` | `1` | same |

response headers:
| Key | A | B | Comparison |
| --- | --- | --- | --- |
| Accept-Ch | `Sec-CH-UA-Arch, Sec-CH-UA-Bitness, Sec-CH-UA-Full-Version, Sec-CH-UA-Full-Version-List, Sec-CH-UA-Model, Sec-CH-UA-WoW64, Sec-CH-UA-Form-Factors, Sec-CH-UA-Platform, Sec-CH-UA-Platform-Version` | `Sec-CH-UA-Arch, Sec-CH-UA-Bitness, Sec-CH-UA-Full-Version, Sec-CH-UA-Full-Version-List, Sec-CH-UA-Model, Sec-CH-UA-WoW64, Sec-CH-UA-Form-Factors, Sec-CH-UA-Platform, Sec-CH-UA-Platform-Version` | same |
| Alt-Svc | `h3=":443"; ma=2592000,h3-29=":443"; ma=2592000` | `h3=":443"; ma=2592000,h3-29=":443"; ma=2592000` | same |
| Cache-Control | `no-cache, no-store, max-age=0, must-revalidate` | `no-cache, no-store, max-age=0, must-revalidate` | same |
| Content-Disposition | `attachment; filename="response.bin"; filename*=UTF-8''response.bin` | `attachment; filename="response.bin"; filename*=UTF-8''response.bin` | same |
| Content-Encoding | `gzip` | `gzip` | same |
| Content-Security-Policy | `require-trusted-types-for 'script';report-uri /_/AiSandboxAngularFrontend/cspreport` | `require-trusted-types-for 'script';report-uri /_/AiSandboxAngularFrontend/cspreport` | same |
| Content-Type | `application/json; charset=utf-8` | `application/json; charset=utf-8` | same |
| Cross-Origin-Opener-Policy | `same-origin-allow-popups` | `same-origin-allow-popups` | same |
| Cross-Origin-Resource-Policy | `same-site` | `same-site` | same |
| Date | `Tue, 20 Oct 2026 03:48:39 GMT` | `Tue, 20 Oct 2026 03:50:33 GMT` | DIFF |
| Expires | `Mon, 01 Jan 1990 00:00:00 GMT` | `Mon, 01 Jan 1990 00:00:00 GMT` | same |
| Permissions-Policy | `ch-ua-arch=*, ch-ua-bitness=*, ch-ua-full-version=*, ch-ua-full-version-list=*, ch-ua-model=*, ch-ua-wow64=*, ch-ua-form-factors=*, ch-ua-platform=*, ch-ua-platform-version=*` | `ch-ua-arch=*, ch-ua-bitness=*, ch-ua-full-version=*, ch-ua-full-version-list=*, ch-ua-model=*, ch-ua-wow64=*, ch-ua-form-factors=*, ch-ua-platform=*, ch-ua-platform-version=*` | same |
| Pragma | `no-cache` | `no-cache` | same |
| Server | `ESF` | `ESF` | same |
| Set-Cookie | `SIDCC=<value sha256:ac8eed7ea737>; expires=<value sha256:58e78bb35a3b>; path=<value sha256:8a5edab28263>; domain=<value sha256:5d59719991ce>; priority=high`; `__Secure-1PSIDCC=<value sha256:5aa030b2e089>; expires=<value sha256:58e78bb35a3b>; path=<value sha256:8a5edab28263>; domain=<value sha256:5d59719991ce>; Secure; HttpOnly; priority=high`; `__Secure-3PSIDCC=<value sha256:40ef84e49aa3>; expires=<value sha256:58e78bb35a3b>; path=<value sha256:8a5edab28263>; domain=<value sha256:5d59719991ce>; Secure; HttpOnly; priority=<value sha256:6ef7c9b15ecd>; SameSite=none` | `SIDCC=<value sha256:5e07dba48dda>; expires=<value sha256:52dc00694ccd>; path=<value sha256:8a5edab28263>; domain=<value sha256:5d59719991ce>; priority=high`; `__Secure-1PSIDCC=<value sha256:d8dbb4ad034e>; expires=<value sha256:52dc00694ccd>; path=<value sha256:8a5edab28263>; domain=<value sha256:5d59719991ce>; Secure; HttpOnly; priority=high`; `__Secure-3PSIDCC=<value sha256:915e2dea3361>; expires=<value sha256:52dc00694ccd>; path=<value sha256:8a5edab28263>; domain=<value sha256:5d59719991ce>; Secure; HttpOnly; priority=<value sha256:6ef7c9b15ecd>; SameSite=none` | DIFF |
| Vary | `Sec-Fetch-Dest, Sec-Fetch-Mode, Sec-Fetch-Site` | `Sec-Fetch-Dest, Sec-Fetch-Mode, Sec-Fetch-Site` | same |
| X-Content-Type-Options | `nosniff` | `nosniff` | same |
| X-Frame-Options | `SAMEORIGIN` | `SAMEORIGIN` | same |
| X-Xss-Protection | `0` | `0` | same |

Set-Cookie theo tên (giá trị thay bằng hash, giữ attributes):
| Cookie name | A | B |
| --- | --- | --- |
| SIDCC | value sha=ac8eed7ea737;  expires=Wed, 20-Oct-2027 03:48:49 GMT; path=/; domain=.google.com; priority=high | value sha=5e07dba48dda;  expires=Wed, 20-Oct-2027 03:50:34 GMT; path=/; domain=.google.com; priority=high |
| __Secure-1PSIDCC | value sha=5aa030b2e089;  expires=Wed, 20-Oct-2027 03:48:49 GMT; path=/; domain=.google.com; Secure; HttpOnly; priority=high | value sha=d8dbb4ad034e;  expires=Wed, 20-Oct-2027 03:50:34 GMT; path=/; domain=.google.com; Secure; HttpOnly; priority=high |
| __Secure-3PSIDCC | value sha=40ef84e49aa3;  expires=Wed, 20-Oct-2027 03:48:49 GMT; path=/; domain=.google.com; Secure; HttpOnly; priority=high; SameSite=none | value sha=915e2dea3361;  expires=Wed, 20-Oct-2027 03:50:34 GMT; path=/; domain=.google.com; Secure; HttpOnly; priority=high; SameSite=none |

Form keys:
| Key | A | B | Comparison |
| --- | --- | --- | --- |
| at | len=42; sha256[:12]=112afd3e4f17 | len=42; sha256[:12]=fec8eab2b30c | DIFF |
| f.req | `<JSON expanded below>` | `<JSON expanded below>` | same |

Envelope: `[[[rpcid, JSON.stringify(argument), null, "generic"]]]`; rpcid khớp query. Form chỉ có `at`, `f.req`.
Argument sau giải mã từng tầng JSON (zero-based, mọi scalar/null):
| Path | A | B | Comparison |
| --- | --- | --- | --- |
| $[0] | null | null | same |
| $[1][0][0] | null | null | same |
| $[1][0][10] | null | null | same |
| $[1][0][11] | null | null | same |
| $[1][0][12] | `86970852-564d-4a92-80cd-98dd8d870177` | `dfde4141-b08f-49ed-ae8d-fb075e6c4d3f` | DIFF |
| $[1][0][13] | `c1ade7d1-be29-43a4-85a9-034a0a61d0d6` | `4c9a40de-96fd-4439-a252-f66744c1bb05` | DIFF |
| $[1][0][1] | null | null | same |
| $[1][0][2] | null | null | same |
| $[1][0][3] | 172976800 | 902307700 | DIFF |
| $[1][0][4] | 1 | 1 | same |
| $[1][0][5] | `HARBOR_SEAL` | `HARBOR_SEAL` | same |
| $[1][0][6] | null | null | same |
| $[1][0][7][0] | null | null | same |
| $[1][0][7][10][0] | len=2382; sha256[:12]=065bb1f3aade | len=2361; sha256[:12]=667daecdb342 | DIFF |
| $[1][0][7][10][1] | 1 | 1 | same |
| $[1][0][7][1] | 22 | 22 | same |
| $[1][0][7][2] | null | null | same |
| $[1][0][7][3] | null | null | same |
| $[1][0][7][4] | null | null | same |
| $[1][0][7][5] | `78e84224-51c6-44e6-ab3c-1033351153bf` | `7f13ac17-918e-4c58-a170-cc2356e3677e` | DIFF |
| $[1][0][7][6] | null | null | same |
| $[1][0][7][7] | null | null | same |
| $[1][0][7][8] | null | null | same |
| $[1][0][7][9] | null | null | same |
| $[1][0][8][0][0][0] | `A watercolor illustration of a blue teacup beside a sunny window` | `A watercolor illustration of a blue teacup beside a sunny window` | same |
| $[1][0][9] | null | null | same |
| $[2] | 1 | 1 | same |
| $[3][0] | null | null | same |
| $[3][10][0] | len=2382; sha256[:12]=065bb1f3aade | len=2361; sha256[:12]=667daecdb342 | DIFF |
| $[3][10][1] | 1 | 1 | same |
| $[3][1] | 22 | 22 | same |
| $[3][2] | null | null | same |
| $[3][3] | null | null | same |
| $[3][4] | null | null | same |
| $[3][5] | `78e84224-51c6-44e6-ab3c-1033351153bf` | `7f13ac17-918e-4c58-a170-cc2356e3677e` | DIFF |
| $[3][6] | null | null | same |
| $[3][7] | null | null | same |
| $[3][8] | null | null | same |
| $[3][9] | null | null | same |
| $[4][0] | `0599eaa6-99df-4e7d-8a6c-f6bc6f617f49` | `ccb5c865-9540-486b-bf8c-d622fde43c99` | DIFF |

Response sau bỏ XSSI/length framing và giải mã payload string:
| Path | A | B | Comparison |
| --- | --- | --- | --- |
| $[0][0] | `wrb.fr` | `wrb.fr` | same |
| $[0][1] | `ogiZ0b` | `ogiZ0b` | same |
| $[0][2] | (absent) | null | DIFF |
| $[0][2][0][0][0] | `80f8f123-7cfd-4836-86a7-b3e58d8fda4f` | (absent) | DIFF |
| $[0][2][0][0][1] | null | (absent) | DIFF |
| $[0][2][0][0][2] | `5027c6b0-6ca6-4276-b11b-f2d16ba2339b` | (absent) | DIFF |
| $[0][2][0][0][3] | null | (absent) | DIFF |
| $[0][2][0][0][4] | null | (absent) | DIFF |
| $[0][2][0][0][5] | null | (absent) | DIFF |
| $[0][2][0][0][6][0][0] | null | (absent) | DIFF |
| $[0][2][0][0][6][0][10] | null | (absent) | DIFF |
| $[0][2][0][0][6][0][11] | `5027c6b0-6ca6-4276-b11b-f2d16ba2339b` | (absent) | DIFF |
| $[0][2][0][0][6][0][12] | null | (absent) | DIFF |
| $[0][2][0][0][6][0][13] | `https://flow-content.google/image/80f8f123-7cfd-4836-86a7-b3e58d8fda4f?Expires=1792489719&KeyName=labs-flow-prod-cdn-key&Signature=[redacted]` | (absent) | DIFF |
| $[0][2][0][0][6][0][14] | 1 | (absent) | DIFF |
| $[0][2][0][0][6][0][15][0] | null | (absent) | DIFF |
| $[0][2][0][0][6][0][15][1] | null | (absent) | DIFF |
| $[0][2][0][0][6][0][15][2][0][0] | `A watercolor illustration of a blue teacup beside a sunny window` | (absent) | DIFF |
| $[0][2][0][0][6][0][15][2][0][1] | null | (absent) | DIFF |
| $[0][2][0][0][6][0][15][2][0][2][0][0][0] | `A watercolor illustration of a blue teacup beside a sunny window` | (absent) | DIFF |
| $[0][2][0][0][6][0][15][3] |  | (absent) | DIFF |
| $[0][2][0][0][6][0][16] | null | (absent) | DIFF |
| $[0][2][0][0][6][0][17] | `80f8f123-7cfd-4836-86a7-b3e58d8fda4f` | (absent) | DIFF |
| $[0][2][0][0][6][0][1] | 172976800 | (absent) | DIFF |
| $[0][2][0][0][6][0][2] | null | (absent) | DIFF |
| $[0][2][0][0][6][0][3] | null | (absent) | DIFF |
| $[0][2][0][0][6][0][4] | null | (absent) | DIFF |
| $[0][2][0][0][6][0][5] | null | (absent) | DIFF |
| $[0][2][0][0][6][0][6] | 1 | (absent) | DIFF |
| $[0][2][0][0][6][0][7] | `A watercolor illustration of a blue teacup beside a sunny window` | (absent) | DIFF |
| $[0][2][0][0][6][0][8] | 31 | (absent) | DIFF |
| $[0][2][0][0][6][0][9] | null | (absent) | DIFF |
| $[0][2][0][0][6][1] | null | (absent) | DIFF |
| $[0][2][0][0][6][2][0] | 1024 | (absent) | DIFF |
| $[0][2][0][0][6][2][1] | 1024 | (absent) | DIFF |
| $[0][2][0][0][6][3] | null | (absent) | DIFF |
| $[0][2][0][0][6][4] | null | (absent) | DIFF |
| $[0][2][0][0][6][5][0] | null | (absent) | DIFF |
| $[0][2][0][0][6][5][1] | `https://flow-content.google/image/80f8f123-7cfd-4836-86a7-b3e58d8fda4f?Expires=1792489719&KeyName=labs-flow-prod-cdn-key&Signature=[redacted]` | (absent) | DIFF |
| $[0][2][1][0][0] | `5027c6b0-6ca6-4276-b11b-f2d16ba2339b` | (absent) | DIFF |
| $[0][2][1][0][1] | null | (absent) | DIFF |
| $[0][2][1][0][2] | null | (absent) | DIFF |
| $[0][2][1][0][3][0] | `Blue teacup by sunny window` | (absent) | DIFF |
| $[0][2][1][0][3][1][0] | 1792468120 | (absent) | DIFF |
| $[0][2][1][0][3][1][1] | 214382000 | (absent) | DIFF |
| $[0][2][1][0][3][2] | null | (absent) | DIFF |
| $[0][2][1][0][3][3] | null | (absent) | DIFF |
| $[0][2][1][0][3][4] | `80f8f123-7cfd-4836-86a7-b3e58d8fda4f` | (absent) | DIFF |
| $[0][2][1][0][3][5] | `0599eaa6-99df-4e7d-8a6c-f6bc6f617f49` | (absent) | DIFF |
| $[0][2][1][0][3][6][0] | 1792468127 | (absent) | DIFF |
| $[0][2][1][0][3][6][1] | 243093000 | (absent) | DIFF |
| $[0][2][1][0][4] | `78e84224-51c6-44e6-ab3c-1033351153bf` | (absent) | DIFF |
| $[0][3] | null | null | same |
| $[0][4] | null | null | same |
| $[0][5] | null | (absent) | DIFF |
| $[0][5][0] | (absent) | 7 | DIFF |
| $[0][5][1] | (absent) | null | DIFF |
| $[0][5][2][0][0] | (absent) | `type.googleapis.com/google.rpc.ErrorInfo` | DIFF |
| $[0][5][2][0][1][0] | (absent) | `PUBLIC_ERROR_UNUSUAL_ACTIVITY` | DIFF |
| $[0][6] | `generic` | `generic` | same |
| $[1][0] | `di` | `di` | same |
| $[1][1] | 10461 | 304 | DIFF |
| $[2][0] | `af.httprm` | `af.httprm` | same |
| $[2][1] | 10460 | 303 | DIFF |
| $[2][2] | `-1651432940628137695` | `8503743426208536473` | DIFF |
| $[2][3] | 24 | 20 | DIFF |
| $[3][0] | `e` | `e` | same |
| $[3][1] | 4 | 4 | same |
| $[3][2] | null | null | same |
| $[3][3] | null | null | same |
| $[3][4] | 1328 | 228 | DIFF |

## Cookie từng tên và tính nhất quán xác thực/session
A: Cookie read và generation giống nguyên byte; thứ tự tên xem bảng dưới.
| Cookie name | length | sha256[:12] | Unchanged in generation |
| --- | --- | --- | --- |
| APISID | 34 | 40fbe1fb7143 | True |
| HSID | 17 | db8dbf2100ed | True |
| SAPISID | 34 | 2abc582e6bdb | True |
| SID | 153 | ca1741c7867c | True |
| SSID | 17 | ec5bbd5acd77 | True |
| __Secure-1PSID | 153 | dc349796908a | True |
| __Secure-1PSIDTS | 77 | 3578d38aa99d | True |
| __Secure-3PSID | 153 | 0a1a17424511 | True |
| __Secure-3PSIDTS | 77 | 3578d38aa99d | True |
| OSID | 153 | fba6e57129a7 | True |
| __Secure-OSID | 153 | dd500e5e3599 | True |

1792468118506296700-000009: SHA1(timestamp + space + SAPISID + space + origin) khớp Authorization = True.
1792468119172976800-000013: SHA1(timestamp + space + SAPISID + space + origin) khớp Authorization = True.
`at` và `f.sid` giữ nguyên giữa read/generate: True.
Challenge có trong anchor: True; token reload khớp cả [1][0][7][10][0] và [3][10][0]: True; hai context giống nguyên khối: True.
Project source-path khớp cả hai context: True.
B: Cookie read và generation giống nguyên byte; thứ tự tên xem bảng dưới.
| Cookie name | length | sha256[:12] | Unchanged in generation |
| --- | --- | --- | --- |
| APISID | 34 | 1ad7ab34ad13 | True |
| HSID | 17 | 631f3efce2e9 | True |
| SAPISID | 34 | 6766a02fd537 | True |
| SID | 153 | 3ba6eb7705dc | True |
| SSID | 17 | 51842679707a | True |
| __Secure-1PSID | 153 | 1457ff8a0d9c | True |
| __Secure-1PSIDTS | 85 | b26f3c73b277 | True |
| __Secure-3PSID | 153 | 6edce46ac90f | True |
| __Secure-3PSIDTS | 85 | b26f3c73b277 | True |
| OSID | 153 | effcc72c80a9 | True |
| __Secure-OSID | 153 | 8b8bf344a37b | True |

1792468233184403100-000003: SHA1(timestamp + space + SAPISID + space + origin) khớp Authorization = True.
1792468233902307700-000007: SHA1(timestamp + space + SAPISID + space + origin) khớp Authorization = True.
`at` và `f.sid` giữ nguyên giữa read/generate: True.
Challenge có trong anchor: True; token reload khớp cả [1][0][7][10][0] và [3][10][0]: True; hai context giống nguyên khối: True.
Project source-path khớp cả hai context: True.

## Diễn giải payload theo code hiện tại
buildGenerateArgument: [0]=null; [1]=một request; [1][0][0,1,2]=null (không reference); [3]=seed từ thời gian; [4]=1 (square); [5]=HARBOR_SEAL; [6]=null; [7]=context; [8][0][0][0]=prompt; [9,10,11]=null; [12,13]=hai UUID. Ở root, [2]=1 là giá trị builder cố định, số ảnh thể hiện bằng độ dài [1]; [3]=context lặp; [4][0]=UUID mới. Context có [0]=null, [1]=22 (toolContextID trong code), [2,3,4]=null, [5]=project, [6,7,8,9]=null, [10]=[token,1]. Ý nghĩa proto của các flag/null/UUID chưa xác minh được từ hai log.

## Những khác biệt cần chú ý
- Phiên reCAPTCHA A=4EDjxXf4knsx6UBM1L2V6cv8; B=NhfdZHzre16wrc2m6BxEdu7J. Hai enterprise.js chỉ khác release path và SRI integrity khi chuẩn hóa hai phần này. Mỗi lần dùng đúng version trả về trong anchor/reload; không thấy trộn phiên hay hardcode version cũ.
- Flow và UA reCAPTCHA khai Chrome 155 nhưng full-version còn 152; reCAPTCHA sec-ch-ua khai 152. A tải ảnh với UA 152. Lệch fingerprint tồn tại chung hai lần: có thể góp tín hiệu rủi ro nhưng chưa chứng minh gây khác biệt kết quả. Source transport mặc định Chrome_152; trace không xác nhận runtime TLS profile của binary.
- Accept-Language ở Flow: A=vi-VN, B=en-US. reCAPTCHA cả hai en-US,en;q=0.9 và hl=en. A thành công dù lệch ngôn ngữ; chưa đủ bằng chứng quy lỗi language.
- nzlxg: A=[888,1,2,2,null,888,[1793871876,115491000]]; B=[50,3,8,1,null,50,[1792554633,296744000]]. Code Credits đọc số đầu: 888/50. Các slot còn lại chưa có schema xác minh; không tự gán tier hay trạng thái ban. B không trả thiếu credit hoặc MODEL_ACCESS_DENIED.
- SAPISIDHASH B giống giữa hai request vì cùng giây Unix; A khác vì qua giây kế. Cả bốn hash đều đúng, không phải lỗi reuse Authorization.
- Google set SIDCC/1PSIDCC/3PSIDCC tại response read/generation; enterprise.js trả EXPIRED cả hai lần. Pattern giống nhau. Snapshot Cookie không mang SIDCC nhưng transport có jar; muốn biết header cuối cùng phải quan sát sau jar.
- Hậu tố timestamp trong at của A trước read khoảng 384 giây, B khoảng 58 giây. Cả hai đọc credit thành công và không có CSRF challenge. Không có bằng chứng at cũ làm B thất bại.

## Hai lần gọi tiếp theo trong trace gốc hiện tại
| ID | UTC | ms | Project | Token SHA256[:12] | Result |
| --- | --- | --- | --- | --- | --- |
| 1792468233902307700-000007 | 2026-10-20T03:50:33.9028408Z | 344 | 7f13ac17-918e-4c58-a170-cc2356e3677e | 667daecdb342 | PUBLIC_ERROR_UNUSUAL_ACTIVITY |
| 1792468239901252800-000012 | 2026-10-20T03:50:39.9012528Z | 420 | 7f13ac17-918e-4c58-a170-cc2356e3677e | 72dc2c3fb764 | PUBLIC_ERROR_UNUSUAL_ACTIVITY |
| 1792468246037873300-000017 | 2026-10-20T03:50:46.0384773Z | 339 | 7f13ac17-918e-4c58-a170-cc2356e3677e | 6aec6a17848c | PUBLIC_ERROR_UNUSUAL_ACTIVITY |

Ba token khác nhau, cùng SAPISID/project và cùng lỗi; generation cách nhau khoảng 5.998 và 6.137 giây. Điều này giảm khả năng một token hỏng đơn lẻ, nhưng chưa tách được account/session/IP/fingerprint/reCAPTCHA release. HTTP trace không xác định retry do FastAPI, scheduler hay người gọi.

## Dữ liệu còn thiếu để kết luận nhân quả
Không có body FastAPI đầu vào hoặc request ID liên kết; tính giống nhau của body đầu vào là thông tin người dùng cung cấp. Hai lần cách nhau khoảng 115 giây, mỗi account dùng session/project khác. Thiếu assessment score/reasons, egress IP/proxy, wire cookies, lịch sử hoạt động và trạng thái eligibility. Kết luận chắc chắn hiện tại: generation của B bị backend từ chối bằng UNUSUAL_ACTIVITY trong luồng token HTTP, chưa quy được cho một key duy nhất.

Để phân biệt: đối chiếu một lần tạo ảnh trong Flow chính thức bằng từng account cùng mạng/thời điểm. Nếu B cũng thất bại trong Flow chính thức thì ưu tiên account/session hoặc môi trường. Nếu B chỉ thất bại qua service, cần request thành công của chính B để so với service, nhất là token provenance và fingerprint. Khi ghi log nên có account_id, project, session hash, captcha provider/token hash, egress/proxy ID và parent request ID; nếu cần wire cookies, chụp sau cookie jar. Không dùng retry liên tục để suy ra score.

Google phân biệt việc có token, tokenProperties.valid, action và riskAnalysis.score: https://docs.cloud.google.com/recaptcha/docs/interpret-assessment-website . Hai trace không chứa assessment nên không đọc ra score của A/B. Comment trong repo mô tả UNUSUAL_ACTIVITY theo token/rate là cách code xử lý, chưa chứng minh tín hiệu nội bộ của Google.

## Chỉ mục source hiện tại
- internal/recaptcha/recaptcha.go:202: HTTPProvider.fetch (release → anchor → reload); chỉ kiểm tra HTTP, parse và độ dài token.
- internal/batchexecute/client.go: Authorization; call/post; buildGenerateArgument; GenerateMedia/retryIfEmpty; Credits; ReasonUnusualActivity.
- internal/engine/engine.go:4784: GenerateImageViaBatch (ensureAffordable → captcha → GenerateMedia → download).
- internal/httpx/client.go:336: doHTTP2 chụp trace trước c.hc.Do; transport có jar; mặc định Chrome_152.
- internal/httpx/trace.go:92: setRequest lưu map header, không capture HTTP/2 wire.
Source là working tree hiện tại; chưa có build commit/hash để chứng minh binary phát log tương ứng chính xác với source này.

## Request CDN chỉ có ở A

URL ảnh lấy từ response generation, media_id=80f8f123-7cfd-4836-86a7-b3e58d8fda4f. GET flow-content.google/image/<media_id>. Cookie và Authorization không có trong snapshot. CDN trả 200 sau khi generation hoàn tất, không phải bước gây lỗi B.

| Query key | A |
| --- | --- |
| Expires | 1792489719 |
| KeyName | labs-flow-prod-cdn-key |
| Signature | sha256[:12]=4f034b426f01 |

request headers:

| Key | Value |
| --- | --- |
| Accept | ["image/*,video/*,*/*"] |
| Accept-Language | ["en-US,en;q=0.9"] |
| Header-Order: | ["host", "sec-ch-ua", "sec-ch-ua-mobile", "sec-ch-ua-platform", "sec-ch-ua-arch", "sec-ch-ua-bitness", "sec-ch-ua-full-version", "user-agent", "accept", "accept-encoding", "accept-language", "authorization", "content-type", "origin", "referer", "cookie", "x-goog-api-key", "priority"] |
| PHeader-Order: | [":method", ":authority", ":scheme", ":path"] |
| Priority | ["u=1, i"] |
| Referer | ["https://flow.google.com/"] |
| Sec-Ch-Ua | ["\"Not(A:Brand\";v=\"99\", \"Google Chrome\";v=\"152\", \"Chromium\";v=\"152\""] |
| Sec-Ch-Ua-Arch | ["\"x86\""] |
| Sec-Ch-Ua-Bitness | ["\"64\""] |
| Sec-Ch-Ua-Full-Version | ["\"152.0.0.0\""] |
| Sec-Ch-Ua-Mobile | ["?0"] |
| Sec-Ch-Ua-Platform | ["\"Windows\""] |
| User-Agent | ["Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/152.0.0.0 Safari/537.36"] |

response headers:

| Key | Value |
| --- | --- |
| Accept-Ranges | ["bytes"] |
| Access-Control-Allow-Origin | ["*"] |
| Access-Control-Expose-Headers | ["Content-Type"] |
| Alt-Svc | ["h3=\":443\"; ma=2592000"] |
| Cache-Control | ["private,max-age=18060"] |
| Content-Length | ["237794"] |
| Content-Type | ["image/jpeg"] |
| Date | ["Tue, 20 Oct 2026 03:48:49 GMT"] |
| Etag | ["\"2b871ac35285b17da41ff13ff8d86346\""] |
| Last-Modified | ["Tue, 20 Oct 2026 03:48:49 GMT"] |
| Server | ["UploadServer"] |
| Via | ["1.1 google"] |
| X-Goog-Generation | ["1792468129395008"] |
| X-Goog-Hash | ["crc32c=nsp3mg==", "md5=K4caw1KFsX2kH/E/+NhjRg=="] |
| X-Goog-Metageneration | ["1"] |
| X-Goog-Storage-Class | ["STANDARD"] |
| X-Goog-Stored-Content-Encoding | ["identity"] |
| X-Goog-Stored-Content-Length | ["237794"] |
| X-Guploader-Customer | ["cloud-storage"] |
| X-Guploader-Request-Class | ["LATENCY_SENSITIVE"] |
| X-Guploader-Request-Result | ["success"] |
| X-Guploader-Upload-Result | ["success"] |
| X-Guploader-Uploadid | ["AP6rU81lq0FX8DwkW8jB5j9ACxVauLVAR7MlSuyA0W7F8eqJyBlov-vFsf-KB75KTKLsVg32"] |

Response: JPEG 237794 byte, 1024×1024 theo media metadata. MD5 thực tế=2b871ac35285b17da41ff13ff8d86346, khớp Etag và X-Goog-Hash md5. Request body=0 byte.

## Chi tiết token và điểm chưa thể suy ra

HTTPProvider hiện tại chỉ tải enterprise.js để đọc release ID rồi GET anchor và POST reload với 8 form key. Luồng này không thực thi bundle JavaScript trong trình duyệt; trace không có request tới gstatic/recaptcha__en.js. Cả A và B cùng luồng này. Parse được chuỗi token dài chưa chứng minh backend đánh giá token hợp lệ, action đúng hoặc score đạt yêu cầu. Không đọc field 120 trong reload thành score hay coi độ dài token khác nhau là lỗi.

Hai vị trí chứa cùng token thuộc cùng một RPC; không phải hai lần gửi RPC. Mẫu không cho thấy token dùng lại giữa hai generation. Trace hiện tại có ba token B khác nhau vẫn bị cùng từ chối.

## Sidecar integrity inventory
| Log | ID | Side | Actual bytes | Logged bytes | Match | SHA256 |
| --- | --- | --- | --- | --- | --- | --- |
| A | 1792468118506296700-000009 | request | 121 | 121 | True | ee4c28bcd6fff827c3b687a196b0cd1686ec3cdbd3b87d776e73c7dbfb1d5643 |
| A | 1792468118506296700-000009 | response | 184 | 184 | True | eb7eca238bcabc62d0f9f9cb4bd0399533d312854b3ca1dc3dd03a0a31d013ba |
| A | 1792468118872689100-000010 | request | 0 | 0 | True | e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 |
| A | 1792468118872689100-000010 | response | 1731 | 1731 | True | 18f1669a67b0e339748a802f4336362d1fcec2b6d174a7e2b0c27132c92569ae |
| A | 1792468118924692400-000011 | request | 0 | 0 | True | e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 |
| A | 1792468118924692400-000011 | response | 55028 | 55028 | True | 87dd0e037b5917c82d98688ba4f589b3c12c6fada5b146b70ff98b7a746a3d0e |
| A | 1792468119011070100-000012 | request | 2140 | 2140 | True | 22f9f7c0d66e85b11b5510ca5d4dc9857353bf84ccef00c2b97fc702ec10a8a7 |
| A | 1792468119011070100-000012 | response | 2881 | 2881 | True | 45fbf258d25746d14922f442c3ae761537d4bab75fded80605071960e99bb5c8 |
| A | 1792468119172976800-000013 | request | 5539 | 5539 | True | f8e6bcd0c906f64255e54fe2ba85f5b11e8d0c8eaa3df3f0d099d823460e57aa |
| A | 1792468119172976800-000013 | response | 1328 | 1328 | True | 7294f85462b5bf487d89696b42586ef545308db79e9a9d98f9e3caef6806c53f |
| A | 1792468129703369800-000014 | request | 0 | 0 | True | e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 |
| A | 1792468129703369800-000014 | response | 237794 | 237794 | True | 6a9b60be1d82194d0a6059321fa60fbdcabaeb406fdaae55ea63808e0e276589 |
| B | 1792468233184403100-000003 | request | 121 | 121 | True | 15807b1541aeecb6bffcb3ac2c16f7e355ce18fab5679f28829683a36afe6c7f |
| B | 1792468233184403100-000003 | response | 181 | 181 | True | db62dabd356772452c6b6f0f1e430f7433248f9655a801b087fe84a22e93ca64 |
| B | 1792468233542803300-000004 | request | 0 | 0 | True | e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 |
| B | 1792468233542803300-000004 | response | 1731 | 1731 | True | 89971b997fcbcfe7aaa4a098df9515fd7d141060c45d005efbabf8034934f55a |
| B | 1792468233699551100-000005 | request | 0 | 0 | True | e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 |
| B | 1792468233699551100-000005 | response | 55268 | 55268 | True | 0c0b2ade4b469b8db7afd76d0b437d75455664948e45aa275448de2b116020d2 |
| B | 1792468233794151800-000006 | request | 2140 | 2140 | True | 2c61dd5c4dfb0b22a24438c84c62233e8418218044967babb5a594b7b250a856 |
| B | 1792468233794151800-000006 | response | 2860 | 2860 | True | 2c57a96937e1305829ab8279ea0cfad7e9854fc4a127a6eca5a2eb4a2911cb0c |
| B | 1792468233902307700-000007 | request | 5497 | 5497 | True | 95e7aa5855fabac05220891803614a9f609e6b531c04fd2b4382541b24a589e1 |
| B | 1792468233902307700-000007 | response | 228 | 228 | True | a8aa7de5c23d71f0c9265a676cd967c852deb65ea16d7fe76520f40e05f6e581 |



## Capture trình duyệt mới của account B

Capture bổ sung là request `ogiZ0b`, cùng `f.sid` và project với lần B lỗi. Đã giải mã body trong attachment. Token/cookie không ghi thô trong báo cáo.

| Trường | Browser capture | Lần B lỗi từ service | Nhận xét |
|---|---|---|---|
| `rpcids` | `ogiZ0b` | `ogiZ0b` | Giống |
| `source-path` | `/project/7f13ac17-918e-4c58-a170-cc2356e3677e` | `/project/7f13ac17-918e-4c58-a170-cc2356e3677e` | Giống |
| `f.sid` | `6903702824746842711` | cùng giá trị | Cùng session page |
| Project trong context | `7f13ac17-918e-4c58-a170-cc2356e3677e` | cùng giá trị | Giống |
| `hl` | `vi` | `en` | Khác locale |
| `bl` | `boq_labs-ai-sandbox-frontend_20261018.05_p0` | `boq_labs-ai-sandbox-frontend_20260922.00_p0` | Browser build mới hơn |
| `_reqid` | `2538976` | `101000` | Khác; mỗi client service bắt đầu counter mới |
| `at` | timestamp `1792468174950` | cùng timestamp | Tại browser capture đã cũ khoảng 529.0 giây |
| Prompt/model/aspect/count | Cùng prompt, `HARBOR_SEAL`, `1`, `1` | giống | Giống |
| Seed | `1785074409` | `902307700` | Mỗi lần tạo khác nhau |
| Captcha token | len 2510, SHA-256[:12] `feb7b0b33efe` | len 2361, SHA-256[:12] `667daecdb342` | Khác; token browser dài hơn 149 ký tự |
| `sec-ch-ua-full-version` | `155.0.8059.80` | `"152.0.0.0"` | Browser nhất quán Chrome 155; service khai 152 |
| Content-Type | `application/x-www-form-urlencoded;charset=utf-8` | `application/x-www-form-urlencoded;charset=UTF-8` | Khác cách viết hoa charset |

Body browser có 5 root slots, một request tạo ảnh, cùng prompt/model/aspect và hai context giống nguyên khối. Token dài 2510 ký tự được đặt tại cả hai vị trí context; token SHA khác token 2361 ký tự của lần service lỗi. Seed và UUID khác theo từng lần gọi.

### Header browser và transport

Header browser trong nội dung người dùng gửi khai `sec-ch-ua-full-version: 155.0.8059.80`, `sec-ch-ua-full-version-list`, `sec-ch-ua-platform-version: 19.0.0`, `sec-ch-ua-wow64`, `sec-ch-ua-model`, `sec-ch-ua-form-factors`. Request service lỗi chỉ có các hint cơ bản trong trace; `sec-ch-ua-full-version` là `152.0.0.0`. Cả hai khai User-Agent Chrome/155. Source ghim TLS profile Chrome_152 tại `internal/httpx/client.go:254`, độc lập với các request header có thể khai Chrome/155. Trace không có TLS fingerprint trên wire, nên đây là chênh lệch cụ thể trong source/header, chưa đủ chứng minh nó gây từ chối.

Browser capture không liệt kê Accept, Accept-Language, Authorization, Cookie, Origin, sec-fetch hay Priority. Chỉ biết các field ấy không có trong đoạn capture người dùng dán; không thể khẳng định trình duyệt thực tế không gửi. Browser response có SIDCC cookies. Không có response body `ogiZ0b` trong attachment để xác nhận data là media hay ErrorInfo.

### Ý nghĩa của capture mới

- Cùng account/session context, project, prompt và logic payload gửi một request HTTP 200 từ browser. Điều này làm giả thuyết lỗi quyền project/model yếu đi, nhưng HTTP 200 chưa chứng minh RPC thành công.
- Browser dùng chính xác `at` của lần service lỗi. Timestamp đó đã cũ khoảng 8 phút 49 giây khi browser capture diễn ra; chưa có response body để xác nhận `at` được chấp nhận. Dữ liệu này khiến giả thuyết “at cũ 58 giây tự nó làm B lỗi” yếu đi.
- Cookie SIDCC trong response browser hết hạn sau `Date` 11 giây. Lần A thành công lệch 10 giây; B lỗi nhanh lệch 1 giây. Đây là tín hiệu gián tiếp phù hợp với browser request có thời gian xử lý generation lâu hơn, nhưng không thay thế response body.
- Đồng thời đổi nhiều yếu tố: token/provider reCAPTCHA, Chrome full version/TLS, client hints, `hl`, build label, reqid, seed, UUID và thời điểm. Capture không cô lập được một yếu tố.

Nếu response body browser có media, bằng chứng sẽ nghiêng mạnh rằng account B/project có thể tạo ảnh bằng browser, còn khác biệt nằm ở đường gửi/token/fingerprint hoặc telemetry của service. Hiện chưa thể tách token do widget thực thi khỏi token lấy bằng HTTP anchor/reload: attachment chỉ có URL reload kề trước và body generation, thiếu body request/response của reload.
