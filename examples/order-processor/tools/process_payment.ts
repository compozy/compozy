#!/usr/bin/env -S deno run --allow-read --allow-net

/**
 * Payment Processor Tool
 *
 * Processes customer payments. This is a mock implementation for demonstration.
 * In production, integrate with actual payment processors like Stripe, PayPal, etc.
 */

interface PaymentInput {
  customerId: string;
  amount: number;
}

interface PaymentOutput {
  status: "success" | "failed" | "pending";
  transactionId: string;
  amount: number;
  currency: string;
  timestamp: string;
  processingFee?: number;
  paymentMethod?: string;
}

async function processPayment(input: PaymentInput): Promise<PaymentOutput> {
  // Generate transaction ID
  const transactionId = `txn_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;

  // Mock payment validation
  if (input.amount <= 0) {
    throw new Error("Invalid payment amount");
  }

  if (input.amount > 10000) {
    throw new Error("Payment amount exceeds maximum limit");
  }

  // Simulate payment processing delay
  await new Promise(resolve => setTimeout(resolve, 200));

  // Mock payment processing logic
  // In this example, 95% of payments succeed
  const shouldSucceed = Math.random() > 0.05;

  if (!shouldSucceed) {
    return {
      status: "failed",
      transactionId,
      amount: input.amount,
      currency: "USD",
      timestamp: new Date().toISOString()
    };
  }

  // Calculate processing fee (2.9% + $0.30 - typical Stripe pricing)
  const processingFee = Math.round((input.amount * 0.029 + 0.30) * 100) / 100;

  console.log(`
💳 PAYMENT PROCESSING
---
Customer ID: ${input.customerId}
Amount: $${input.amount}
Transaction ID: ${transactionId}
Processing Fee: $${processingFee}
Status: Processing...
---
✅ Payment processed successfully
`);

  return {
    status: "success",
    transactionId,
    amount: input.amount,
    currency: "USD",
    timestamp: new Date().toISOString(),
    processingFee,
    paymentMethod: "credit_card" // Mock payment method
  };
}

// Main execution
if (import.meta.main) {
  try {
    const inputText = await new Promise<string>((resolve) => {
      const chunks: string[] = [];
      const decoder = new TextDecoder();

      const reader = Deno.stdin.readable.getReader();

      const pump = async (): Promise<void> => {
        const { done, value } = await reader.read();
        if (done) {
          resolve(chunks.join(''));
          return;
        }
        chunks.push(decoder.decode(value));
        return pump();
      };

      pump();
    });

    const input: PaymentInput = JSON.parse(inputText);
    const result = await processPayment(input);

    console.log(JSON.stringify(result, null, 2));
  } catch (error) {
    console.error(JSON.stringify({
      error: true,
      message: error.message,
      type: "payment_processing_error"
    }));
    Deno.exit(1);
  }
}
