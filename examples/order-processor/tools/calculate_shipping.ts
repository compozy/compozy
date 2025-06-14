#!/usr/bin/env -S deno run --allow-read --allow-net

/**
 * Shipping Calculator Tool
 *
 * Calculates shipping costs and delivery estimates based on items and destination.
 * This is a mock implementation for demonstration purposes.
 */

interface ShippingInput {
    items: Array<{
        productId: string;
        name: string;
        quantity: number;
        price: number;
    }>;
    address: {
        street: string;
        city: string;
        zipCode: string;
        country: string;
    };
}

interface ShippingOutput {
    cost: number;
    method: string;
    estimatedDelivery: string;
    carrier: string;
    trackingAvailable: boolean;
}

export default function run(input: ShippingInput): ShippingOutput {
    // Calculate total weight (mock - assume 1 lb per item for simplicity)
    const totalWeight = input.items.reduce((sum, item) => sum + item.quantity, 0);

    // Calculate total value
    const totalValue = input.items.reduce((sum, item) => sum + item.quantity * item.price, 0);

    // Base shipping rates by country/region
    const shippingRates = {
        US: { base: 5.99, perPound: 1.5, businessDays: 3 },
        CA: { base: 8.99, perPound: 2.0, businessDays: 5 },
        UK: { base: 12.99, perPound: 2.5, businessDays: 7 },
        default: { base: 15.99, perPound: 3.0, businessDays: 10 },
    };

    // Get shipping rate for destination country
    const rate =
        shippingRates[input.address.country as keyof typeof shippingRates] || shippingRates.default;

    // Calculate shipping cost
    let shippingCost = rate.base + totalWeight * rate.perPound;

    // Free shipping for orders over $100
    if (totalValue >= 100) {
        shippingCost = 0;
    }

    // Expedited shipping for high-value orders
    let shippingMethod = "Standard";
    let businessDays = rate.businessDays;
    let carrier = "USPS";

    if (totalValue >= 500) {
        shippingMethod = "Expedited";
        businessDays = Math.max(1, Math.floor(businessDays / 2));
        carrier = "FedEx";
        if (shippingCost > 0) {
            shippingCost += 10; // Expedited surcharge
        }
    }

    // Calculate estimated delivery date
    const deliveryDate = new Date();
    deliveryDate.setDate(deliveryDate.getDate() + businessDays);

    // Skip weekends (simple approximation)
    if (deliveryDate.getDay() === 0) deliveryDate.setDate(deliveryDate.getDate() + 1); // Sunday -> Monday
    if (deliveryDate.getDay() === 6) deliveryDate.setDate(deliveryDate.getDate() + 2); // Saturday -> Monday



    return {
        cost: Math.round(shippingCost * 100) / 100, // Round to 2 decimal places
        method: shippingMethod,
        estimatedDelivery: deliveryDate.toLocaleDateString("en-US", {
            weekday: "long",
            year: "numeric",
            month: "long",
            day: "numeric",
        }),
        carrier,
        trackingAvailable: true,
    };
}

// Alternative export for compatibility
export { run };
