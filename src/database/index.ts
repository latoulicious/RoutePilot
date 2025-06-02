import { drizzle } from 'drizzle-orm/node-postgres'
import { Pool } from 'pg'
import * as fs from 'fs'
import * as schema from './schema'
import postgres from 'postgres'

const databaseUrl = process.env.DATABASE_URL
if (!databaseUrl) {
  throw new Error('DATABASE_URL environment variable is not set')
}

// Disable prefetch as it is not supported for "Transaction" pool mode
export const client = postgres(databaseUrl, { prepare: false })

const sslCertPath = process.env.PGSSLROOTCERT
if (!sslCertPath) {
  throw new Error('PGSSLROOTCERT environment variable is not set')
}
const sslCert = fs.readFileSync(sslCertPath).toString()

const pool = new Pool({
  connectionString: databaseUrl,
  ssl: {
    ca: sslCert,
    rejectUnauthorized: true, // This is now safe!
  },
})

export const db = drizzle(pool, { schema })
