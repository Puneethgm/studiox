// Custom dev/local server — plain `next dev` has no way to see the raw
// TCP connection, so the Go API behind it only ever saw requests coming
// from this process itself (127.0.0.1/::1), never the real browser's IP
// (see internal/platform/httpx.ClientIP, which reads X-Forwarded-For).
// This sets that header from the actual socket before Next.js handles
// the request, so it survives through middleware and the /api/* rewrite
// unchanged, exactly like a reverse proxy (nginx, in production) already
// does for the deployed stack.
const { createServer } = require('http');
const next = require('next');

const port = parseInt(process.env.PORT || '3010', 10);
const dev = process.env.NODE_ENV !== 'production';
const app = next({ dev });
const handle = app.getRequestHandler();

app.prepare().then(() => {
  createServer((req, res) => {
    if (!req.headers['x-forwarded-for']) {
      const remoteAddr = req.socket.remoteAddress;
      if (remoteAddr) {
        // Strip the IPv4-in-IPv6 "::ffff:" prefix dual-stack sockets add —
        // httpx.ClientIP normalizes this too, but no reason to send it
        // mangled in the first place.
        req.headers['x-forwarded-for'] = remoteAddr.replace(/^::ffff:/, '');
      }
    }
    handle(req, res);
  }).listen(port, () => {
    console.log(`> Ready on http://localhost:${port} (custom server, forwards real client IP)`);
  });
});
