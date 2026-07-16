import { Cart, CheckoutOrchestrator } from "./checkout-orchestrator.js";

export async function placeOrder(orchestrator: CheckoutOrchestrator, cart: Cart): Promise<string> {
  await orchestrator.validate(cart);
  const reservationID = await orchestrator.reserve(cart);
  const chargeID = await orchestrator.charge(cart.customerID, reservationID);
  return orchestrator.createOrder(cart, chargeID);
}
