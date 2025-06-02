export interface Config {
    env: 'development' | 'production' | 'test'
    port: number
    database: string
}

export function getConfig(): Config {
    return {
        env: process.env.NODE_ENV as 'development' | 'production' | 'test',
        port: Number(process.env.PORT) || 3000,
        database: process.env.DATABASE_URL || '',
    }
}