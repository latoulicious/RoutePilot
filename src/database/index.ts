import { drizzle } from 'drizzle-orm/pglite'
import { PGlite } from '@electric-sql/pglite'
import * as schema from './schema'

const dbFile = Bun.env.DATABASE_FILE || './local.db'

const sqlite = await PGlite.create(dbFile)
export const db = drizzle(sqlite, { schema })
