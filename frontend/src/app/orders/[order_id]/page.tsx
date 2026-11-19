import {notFound} from 'next/navigation';
import {commerceGetOrder} from '@/services/generated/commerce/commerce';
import {OrderDetail} from '@/features/commerce/order_detail';

interface OrderDetailPageProps {
  params: Promise<{
    order_id: string;
  }>;
}

export default async function OrderDetailPage({params}: OrderDetailPageProps) {
  const {order_id: orderId} = await params;

  let order: CommerceApi.OrderResponse | null = null;
  try {
    order = await commerceGetOrder({id: orderId});
  } catch {
    notFound();
  }

  if (!order) {
    notFound();
  }

  return (
    <div className="container mx-auto px-4 py-8 max-w-4xl space-y-6">
      <OrderDetail order={order} />
    </div>
  );
}
