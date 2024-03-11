import './wasm_exec.js';
import type { InvoiceValue } from './schemas/bill/types';

type GOBLSchema = InvoiceValue;

export abstract class BaseSchema<T extends GOBLSchema> {
  protected _value: T;
  private wasmInstance: WebAssembly.Exports | null = null;

  SCHEMA: string = 'http://json-schema.org/draft/2020-12/schema';

  abstract SCHEMA_ID: string;

  constructor(value: T) {
    this._value = value;
    this.loadWasm().catch(() => {});
  }

  private async loadWasm(): Promise<void> {
    try {
      const response = await fetch('https://cdn.gobl.org/cli/gobl.0.64.0.wasm');
      const wasmBuffer = await response.arrayBuffer();

      const go = new Go();
      const importObject = go.importObject;

      console.log({ importObject });

      const result = await WebAssembly.instantiate(wasmBuffer, importObject);

      go.run(result.instance).catch(() => {});

      this.wasmInstance = result.instance.exports;
    } catch (error) {
      console.error('Error loading Wasm:', error);
    }
  }

  /**
   * @description Casts root value to JSON.
   */
  public toJSON(): string {
    return JSON.stringify(this._value);
  }

  public callWasmMethod(): void {
    if (this.wasmInstance !== null) {
      // console.log(this.wasmInstance);
      console.log('wasm loaded');
      // Call Wasm methods using this.wasmInstance.methodName
    } else {
      console.error('Wasm not loaded.');
    }
  }
}
