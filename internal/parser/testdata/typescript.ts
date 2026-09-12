export interface User {
  id: number;
  name: string;
}

export type ID = string;

export class Service {
  constructor(private readonly url: string) {}

  run(x: number): string {
    if (x > 0) {
      return this.format(x);
    }
    return "";
  }

  private format(x: number) { return String(x); }

  public static async create(url: string): Promise<Service> {
    return new Service(url);
  }

  get size() { return 0; }
}

export function makeUser(name: string): User {
  return { id: 1, name };
}

const handlers = {
  onClick(event) {
    doSomething(event);
  },
};
