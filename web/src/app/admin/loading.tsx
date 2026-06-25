import { Skeleton } from '@/components/ui/AppleSkeleton';

export default function AdminLoading() {
  return (
    <div className="flex flex-col gap-4 p-6">
      <Skeleton className="w-[200px] h-7" />
      <Skeleton className="w-full h-[200px]" />
      <Skeleton className="w-[60%] h-5" />
    </div>
  );
}
