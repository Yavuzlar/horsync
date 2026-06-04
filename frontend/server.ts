import express from "express";
import { createServer as createViteServer } from "vite";
import path from "path";
import { fileURLToPath } from "url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

async function startServer() {
  const app = express();
  const PORT = 3000;

  app.use(express.json());

  // API Routes
  app.get("/api/health", (req, res) => {
    res.json({ status: "ok", version: "1.2.4-beta" });
  });

  app.get("/api/stats", (req, res) => {
    res.json({
      cpu: "24%",
      ram: "6.2 GB",
      storage: "1.2 TB",
      throughput: "850 MB/s",
      uptime: "14D 06H 22M",
      status: "OPTIMAL"
    });
  });

  app.get("/api/performance", (req, res) => {
    res.json([
      { time: '10:00', speed: 120, ram: 4.2 },
      { time: '10:05', speed: 250, ram: 4.5 },
      { time: '10:10', speed: 480, ram: 5.1 },
      { time: '10:15', speed: 320, ram: 4.8 },
      { time: '10:20', speed: 850, ram: 6.2 },
      { time: '10:25', speed: 640, ram: 5.8 },
      { time: '10:30', speed: 720, ram: 6.0 },
      { time: '10:35', speed: 910, ram: 6.5 },
    ]);
  });

  app.get("/api/nodes", (req, res) => {
    res.json([
      { id: 'NX-01-DSK', type: 'DESKTOP', location: 'FRANKFURT, DE', status: 'active', ip: '192.168.1.104', load: '12%', uptime: '14D' },
      { id: 'NX-02-SRV', type: 'SERVER', location: 'SINGAPORE, SG', status: 'active', ip: '10.0.0.5', load: '45%', uptime: '122D' },
      { id: 'NX-03-LPT', type: 'LAPTOP', location: 'NEW YORK, US', status: 'warning', ip: '172.16.0.42', load: '88%', uptime: '2D' },
      { id: 'NX-04-SRV', type: 'SERVER', location: 'LONDON, UK', status: 'offline', ip: '192.168.2.10', load: '0%', uptime: '0D' },
    ]);
  });

  app.get("/api/rules", (req, res) => {
    res.json([
      { id: 1, name: 'AUTO_ENCRYPT_FINANCIALS', desc: 'Automatically encrypt any PDF containing financial keywords using AES-256.', active: true, lastTriggered: '2 MINS AGO', totalRuns: 1242 },
      { id: 2, name: 'WIPE_EXIF_METADATA', desc: 'Strip metadata from all uploaded images before syncing to global nodes.', active: true, lastTriggered: '1 HR AGO', totalRuns: 8421 },
      { id: 3, name: 'COLD_STORAGE_ARCHIVE', desc: 'Move files untouched for 90 days to deep glacier storage automatically.', active: false, lastTriggered: 'NEVER', totalRuns: 0 },
      { id: 4, name: 'INSTANT_SYNC_PRIORITY', desc: 'Prioritize syncing of files under 10MB across all nodes for low latency.', active: true, lastTriggered: 'JUST NOW', totalRuns: 45210 },
    ]);
  });

  app.get("/api/files", (req, res) => {
    res.json([
      { id: 1, name: 'Q4_FINANCIAL_REPORT.PDF', type: 'pdf', size: '4.2 MB', status: ['ENCRYPTED'], date: '2 MINS AGO' },
      { id: 2, name: 'PRODUCT_SHOOT_RAW.ZIP', type: 'archive', size: '12.4 GB', status: ['EXIF_CLEANED', 'ON_DEMAND'], date: '1 HR AGO' },
      { id: 3, name: 'CEO_KEYNOTE_DRAFT.MP4', type: 'video', size: '2.1 GB', status: ['ENCRYPTED'], date: '3 HRS AGO' },
      { id: 4, name: 'DESIGN_SYSTEM_V2.FIG', type: 'design', size: '845 MB', status: ['ON_DEMAND'], date: 'YESTERDAY' },
      { id: 5, name: 'CLIENT_CONTRACTS_2026.ZIP', type: 'archive', size: '1.2 GB', status: ['ENCRYPTED', 'EXIF_CLEANED'], date: 'YESTERDAY' },
    ]);
  });

  app.get("/api/security/logs", (req, res) => {
    res.json([
      { id: 1, event: 'KEY_ROTATION_SUCCESS', type: 'success', time: '2 MINS AGO', detail: 'Primary AES-256 key rotated successfully.' },
      { id: 2, event: 'UNAUTHORIZED_ACCESS_BLOCKED', type: 'error', time: '15 MINS AGO', detail: 'IP 192.168.1.205 blocked after 5 failed attempts.' },
      { id: 3, event: 'VAULT_INTEGRITY_CHECK', type: 'info', time: '1 HR AGO', detail: 'All 12,402 encrypted blocks verified.' },
    ]);
  });

  // Vite middleware for development
  if (process.env.NODE_ENV !== "production") {
    const vite = await createViteServer({
      server: { middlewareMode: true },
      appType: "spa",
    });
    app.use(vite.middlewares);
  } else {
    const distPath = path.join(process.cwd(), 'dist');
    app.use(express.static(distPath));
    app.get('*', (req, res) => {
      res.sendFile(path.join(distPath, 'index.html'));
    });
  }

  app.listen(PORT, "0.0.0.0", () => {
    console.log(`Server running on http://localhost:${PORT}`);
  });
}

startServer();
