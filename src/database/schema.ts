import { pgTable, varchar, jsonb, timestamp, serial } from 'drizzle-orm/pg-core'
import { z } from 'zod'

export const routeConfigs = pgTable('route_configs', {
  id: serial('id').primaryKey(),
  path: varchar('path', { length: 255 }).notNull(),
  method: varchar('method', { length: 10 }).notNull(),
  config: jsonb('config').notNull(), // stored as JSON
  environment: varchar('environment', { length: 50 }).default('production'),
  createdAt: timestamp('created_at').defaultNow(),
  updatedAt: timestamp('updated_at').defaultNow(),
})
