// Intentional type error for diagnostics testing
const x: number = "hello";

export function broken(n: number): string {
  return n;  // Type 'number' is not assignable to type 'string'
}
