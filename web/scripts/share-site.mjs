// Non-secret build-time branding for the Go server's public link previews.
import { writeFile } from 'node:fs/promises';
import { loadEnv } from 'vite';
const env = loadEnv(process.env.NODE_ENV || 'production', process.cwd(), 'VITE_');

// This deployment's own origin is the default rather than empty. Upstream leaves
// it blank because a self-hoster must supply their own; here the site has one
// fixed public origin, hardcoded the same way Seo.svelte does.
//
// It has to be absolute: the Go renderer builds og:image and og:url from it, and
// a chat app given a relative image URL simply shows no preview — with nothing
// logged anywhere to say why.
await writeFile('build/share-site.json', JSON.stringify({
  name: env.VITE_SITE_NAME || 'Ridgeline',
  url: (env.VITE_SITE_URL || 'https://ridgeline.ve7kod.ca').replace(/\/+$/, ''),
}) + '\n');
