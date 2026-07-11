# Horsync Frontend

React + TypeScript SPA for the Horsync P2P synchronization platform.

## Tech Stack

- **React 19** — UI framework
- **TypeScript** — Type safety
- **Vite 6** — Build tool
- **Tailwind CSS 4** — Styling
- **Recharts** — Performance graphs
- **Lucide React** — Icons
- **Motion** — Animations

## Getting Started

```bash
cd frontend
npm install
npm run dev
```

The development server starts at `http://localhost:3000` and expects the backend API at `http://localhost:3001`.

## Available Scripts

| Command           | Description                       |
|-------------------|-----------------------------------|
| `npm run dev`     | Start Vite dev server             |
| `npm run build`   | Production build                  |
| `npm run preview` | Preview production build          |
| `npm run lint`    | TypeScript type check (`tsc`)     |
| `npm run clean`   | Remove `dist/` directory          |
| `npm run dev:mock`| Start with mock API server (tsx)  |

## Project Structure

```
frontend/src/
├── components/         # React UI components
│   ├── MainHub.tsx     # Dashboard with stats & performance
│   ├── FileExplorer.tsx # File upload & management
│   ├── GlobalNodesView.tsx # P2P node topology
│   ├── SecurityVaultView.tsx # Encryption vault UI
│   ├── SettingsView.tsx # Instance settings
│   ├── LoginView.tsx   # Authentication
│   ├── NodeActivity.tsx # Node activity feed
│   ├── Sidebar.tsx     # Navigation sidebar
│   └── AutomationRulesView.tsx # Rule management
├── lib/                # Shared types, utilities
│   ├── types.ts        # TypeScript interfaces
│   ├── i18n.tsx        # Internationalization
│   ├── utils.ts        # Helper functions
│   └── upload.ts       # Upload utilities
├── services/           # API client
│   └── api.ts          # REST API client
├── App.tsx             # Root component
├── main.tsx            # Entry point
└── index.css           # Global styles
```

## API Client

The frontend communicates with the Horsync backend via REST API. The API client is in `src/services/api.ts`.

Authentication uses Bearer tokens stored in `localStorage` under the key `yavuzlar_auth_token`.
