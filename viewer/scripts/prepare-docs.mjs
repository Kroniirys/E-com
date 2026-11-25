import { createRequire } from 'node:module'
import { copyFile, mkdir } from 'node:fs/promises'
import { dirname, join } from 'node:path'
const require = createRequire(import.meta.url)
const source = dirname(require.resolve('swagger-ui-dist/package.json'))
await mkdir('public/api-docs', { recursive: true })
for (const name of ['swagger-ui-bundle.js', 'swagger-ui.css', 'favicon-32x32.png']) await copyFile(join(source, name), join('public/api-docs', name))
