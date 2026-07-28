interface StoreSubscription {
  unsubscribe: () => void;
}

interface AwaitStoreRequestOptions<
  TAccepted extends { requestId: number },
  TSettled extends { requestId: number },
  TResult,
> {
  notAcceptedMessage: string;
  request: () => void;
  resolveSettlement: (settlement: TSettled) => TResult;
  subscribeAccepted: (listener: (event: TAccepted) => void) => StoreSubscription;
  subscribeSettled: (listener: (event: TSettled) => void) => StoreSubscription;
}

/** Adapts a synchronously accepted, id-fenced store request to a caller-facing Promise. */
export function awaitStoreRequest<
  TAccepted extends { requestId: number },
  TSettled extends { requestId: number },
  TResult,
>({
  notAcceptedMessage,
  request,
  resolveSettlement,
  subscribeAccepted,
  subscribeSettled,
}: AwaitStoreRequestOptions<TAccepted, TSettled, TResult>): Promise<TResult> {
  return new Promise<TResult>((resolve, reject) => {
    let requestId: number | undefined;
    let accepted: StoreSubscription | undefined;
    let settled: StoreSubscription | undefined;
    const cleanup = () => {
      accepted?.unsubscribe();
      settled?.unsubscribe();
    };

    try {
      accepted = subscribeAccepted(event => {
        requestId ??= event.requestId;
      });
      settled = subscribeSettled(event => {
        if (event.requestId !== requestId) return;
        cleanup();
        try {
          resolve(resolveSettlement(event));
        } catch (error) {
          reject(error);
        }
      });

      request();
      if (requestId === undefined) {
        cleanup();
        reject(new Error(notAcceptedMessage));
      }
    } catch (error) {
      cleanup();
      reject(error);
    }
  });
}
