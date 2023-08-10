export type Metadata = {
    language: string
    kernelspec: {
        language: string
    }
    language_info: {
        name: string
    }
}

export type WorksheetData = {
    cells: Cell[]
}

export type Output = {
    output_type: string
    stream?: string
    name?: string
    data?: {[key: string]: string}
    traceback?: string[]
    text?: string
}

export type Cell = {
    cell_type: "markdown" | "heading" | "raw" | "code"
    source: string
    level?: number
    language?: string
    outputs?: Output[]
    prompt_number?: number
    execution_count: number
    input?: string[]
}
