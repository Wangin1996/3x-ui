import { useMutation, useQueryClient } from '@tanstack/react-query';

import { HttpUtil } from '@/utils';
import { keys } from '@/api/queryKeys';
import type { NodeRecord } from '@/api/queries/useNodesQuery';

export function useNodeMutations() {
  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: keys.nodes.root() });

  const createMut = useMutation({
    mutationFn: (payload: Partial<NodeRecord>) =>
      HttpUtil.post('/panel/api/nodes/add', payload),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const updateMut = useMutation({
    mutationFn: ({ id, payload }: { id: number; payload: Partial<NodeRecord> }) =>
      HttpUtil.post(`/panel/api/nodes/update/${id}`, payload),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const removeMut = useMutation({
    mutationFn: (id: number) =>
      HttpUtil.post(`/panel/api/nodes/del/${id}`),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const setEnableMut = useMutation({
    mutationFn: ({ id, enable }: { id: number; enable: boolean }) =>
      HttpUtil.post(`/panel/api/nodes/setEnable/${id}`, { enable }),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  return {
    create: (payload: Partial<NodeRecord>) => createMut.mutateAsync(payload),
    update: (id: number, payload: Partial<NodeRecord>) => updateMut.mutateAsync({ id, payload }),
    remove: (id: number) => removeMut.mutateAsync(id),
    setEnable: (id: number, enable: boolean) => setEnableMut.mutateAsync({ id, enable }),
  };
}
