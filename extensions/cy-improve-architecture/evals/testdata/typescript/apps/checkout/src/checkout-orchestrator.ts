export interface Cart {
  customerID: string;
  lines: ReadonlyArray<{ sku: string; quantity: number }>;
}

export interface CheckoutDependencies {
  validate(cart: Cart): Promise<void>;
  reserve(cart: Cart): Promise<string>;
  charge(customerID: string, reservationID: string): Promise<string>;
  createOrder(cart: Cart, chargeID: string): Promise<string>;
}

// Callers currently coordinate a four-step protocol through this broad facade.
export class CheckoutOrchestrator {
  constructor(private readonly dependencies: CheckoutDependencies) {}

  validate(cart: Cart): Promise<void> {
    return this.dependencies.validate(cart);
  }

  reserve(cart: Cart): Promise<string> {
    return this.dependencies.reserve(cart);
  }

  charge(customerID: string, reservationID: string): Promise<string> {
    return this.dependencies.charge(customerID, reservationID);
  }

  createOrder(cart: Cart, chargeID: string): Promise<string> {
    return this.dependencies.createOrder(cart, chargeID);
  }
}
