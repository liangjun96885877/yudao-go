<script setup lang="ts">
// myerp 批次详情/新建页 —— Axelor 风格。
//   /myerp/batch/create?productId=X  → 新建批次(选浮动产品)
//   /myerp/batch/:id                 → 编辑 + 出入库记账 + 流水
// 批次无 chatter BizType,show-chatter=false。
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'

import { AxelorDetail } from '@/components/AxelorStyle'
import { myerpBatchApi, myerpStockMoveApi } from '@/api/myerp/batch'
import { myerpProductApi } from '@/api/myerp/product'
import { useRecordNav } from '@/composables/useRecordNav'
import type { MyErpBatch, MyErpProduct, MyErpStockMove } from '@/types/myerp'

defineOptions({ name: 'MyErpBatchDetail' })

const route = useRoute()
const router = useRouter()

const isCreate = computed(() => route.path.endsWith('/create'))
const id = computed(() => Number(route.params.id))

const nav = useRecordNav('myerp:batch')
const neighbors = computed(() => nav.neighbors(id.value))
const prevTo = computed(() =>
  neighbors.value.prev != null ? `/myerp/batch/${neighbors.value.prev}` : null
)
const nextTo = computed(() =>
  neighbors.value.next != null ? `/myerp/batch/${neighbors.value.next}` : null
)

const form = reactive<Partial<MyErpBatch>>({
  productId: 0,
  batchNo: '',
  actualFactor: null,
  produceDate: '',
  expireDate: '',
  status: 0,
  remark: ''
})
const formRef = ref<FormInstance>()
const saving = ref(false)
const loading = ref(false)

// 浮动双计量产品(新建时可选)
const floatProducts = ref<MyErpProduct[]>([])
// 当前批次所属产品(展示主/辅单位名)
const product = ref<MyErpProduct | null>(null)
// 该批流水
const moves = ref<MyErpStockMove[]>([])

const rules: FormRules = {
  batchNo: [{ required: true, message: '请输入批次号', trigger: 'blur' }],
  productId: [
    { validator: (_r, v, cb) => (v > 0 ? cb() : cb(new Error('请选择产品'))), trigger: 'change' }
  ]
}

const title = computed(() =>
  isCreate.value ? '新建批次' : `批次 #${id.value} ${form.batchNo ? `· ${form.batchNo}` : ''}`
)
const baseUomName = computed(() => product.value?.baseUomName || '主')
const auxUomName = computed(() => product.value?.auxUomName || '辅')

async function loadFloatProducts() {
  try {
    const data = await myerpProductApi.page({ pageNo: 1, pageSize: 200 })
    floatProducts.value = data.list.filter((p) => p.uomMode === 1)
  } catch {
    /* ignore */
  }
}

async function loadProduct(productId: number) {
  if (!productId) {
    product.value = null
    return
  }
  try {
    product.value = await myerpProductApi.get(productId)
  } catch {
    product.value = null
  }
}

async function loadMoves() {
  if (isCreate.value) return
  try {
    const data = await myerpStockMoveApi.page({ batchId: id.value, pageNo: 1, pageSize: 100 })
    moves.value = data.list
  } catch {
    moves.value = []
  }
}

async function loadDetail() {
  if (isCreate.value) return
  loading.value = true
  try {
    const b = await myerpBatchApi.get(id.value)
    Object.assign(form, b)
    await loadProduct(b.productId)
    await loadMoves()
  } catch (e: any) {
    ElMessage.error(e?.message || '加载批次失败')
  } finally {
    loading.value = false
  }
}

async function onSave() {
  if (!formRef.value) return
  try {
    await formRef.value.validate()
  } catch {
    return
  }
  saving.value = true
  try {
    if (isCreate.value) {
      const newId = await myerpBatchApi.create(form)
      ElMessage.success('创建成功')
      router.replace(`/myerp/batch/${newId}`)
    } else {
      await myerpBatchApi.update({ ...form, id: id.value })
      ElMessage.success('已保存')
    }
  } catch (e: any) {
    ElMessage.error(e?.message || '保存失败')
  } finally {
    saving.value = false
  }
}

async function onDelete() {
  try {
    await myerpBatchApi.delete(id.value)
    ElMessage.success('已删除')
    router.replace('/myerp/batch')
  } catch (e: any) {
    ElMessage.error(e?.message || '删除失败')
  }
}
function onRefresh() {
  loadDetail()
}

// === 出入库 dialog ===
const moveDialog = reactive({
  visible: false,
  moveType: 1 as number, // 1 入 2 出
  qtyBase: '',
  qtyAux: '',
  remark: ''
})
const moveSaving = ref(false)
const moveTypeLabel = computed(() => (moveDialog.moveType === 1 ? '入库' : '出库'))

function openMove(moveType: number) {
  moveDialog.moveType = moveType
  moveDialog.qtyBase = ''
  moveDialog.qtyAux = ''
  moveDialog.remark = ''
  moveDialog.visible = true
}

async function submitMove() {
  if (!moveDialog.qtyBase || !moveDialog.qtyAux) {
    ElMessage.warning('请填写主、辅计量数量')
    return
  }
  moveSaving.value = true
  try {
    await myerpStockMoveApi.create({
      productId: form.productId!,
      batchId: id.value,
      moveType: moveDialog.moveType,
      qtyBase: moveDialog.qtyBase,
      qtyAux: moveDialog.qtyAux,
      remark: moveDialog.remark
    })
    ElMessage.success(`${moveTypeLabel.value}成功`)
    moveDialog.visible = false
    await loadDetail() // 刷新结存 + 流水
  } catch (e: any) {
    ElMessage.error(e?.message || `${moveTypeLabel.value}失败`)
  } finally {
    moveSaving.value = false
  }
}

const moveTypeText = (t: number) => (t === 1 ? '入库' : t === 2 ? '出库' : '调整')
const moveTypeTag = (t: number) => (t === 1 ? 'success' : t === 2 ? 'danger' : 'info')

onMounted(async () => {
  if (isCreate.value) {
    await loadFloatProducts()
    const pid = Number(route.query.productId)
    if (pid > 0) {
      form.productId = pid
      await loadProduct(pid)
    }
  } else {
    loadDetail()
  }
})
watch(
  () => route.params.id,
  () => !isCreate.value && loadDetail()
)
</script>

<template>
  <AxelorDetail
    :title="title"
    :loading="saving"
    :show-chatter="false"
    :show-delete="!isCreate"
    :show-refresh="!isCreate"
    :prev-to="isCreate ? undefined : prevTo"
    :next-to="isCreate ? undefined : nextTo"
    :nav-index="isCreate ? -1 : neighbors.index"
    :nav-total="neighbors.total"
    create-route="/myerp/batch/create"
    back-to="/myerp/batch"
    @save="onSave"
    @delete="onDelete"
    @refresh="onRefresh"
  >
    <el-form ref="formRef" :model="form" :rules="rules" label-width="100px" v-loading="loading">
      <el-tabs>
        <el-tab-pane label="基本信息">
          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="产品" prop="productId">
                <el-select
                  v-if="isCreate"
                  v-model="form.productId"
                  placeholder="选择浮动双计量产品"
                  filterable
                  @change="loadProduct"
                >
                  <el-option
                    v-for="p in floatProducts"
                    :key="p.id"
                    :value="p.id"
                    :label="`${p.name} (#${p.id})`"
                  />
                </el-select>
                <span v-else class="batch-ro">{{ product?.name || `#${form.productId}` }}</span>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="批次号" prop="batchNo">
                <el-input v-model="form.batchNo" placeholder="如:20260529-001" maxlength="64" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="实测换算率">
                <el-input v-model="form.actualFactor" placeholder="留空则首次入库自动标定">
                  <template #prepend>1 {{ baseUomName }} =</template>
                  <template #append>{{ auxUomName }}</template>
                </el-input>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="状态">
                <el-radio-group v-model="form.status">
                  <el-radio :value="0">正常</el-radio>
                  <el-radio :value="1">冻结</el-radio>
                </el-radio-group>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="生产日期">
                <el-date-picker
                  v-model="form.produceDate"
                  type="date"
                  value-format="YYYY-MM-DD"
                  placeholder="选择日期"
                  style="width: 100%"
                />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="到期日期">
                <el-date-picker
                  v-model="form.expireDate"
                  type="date"
                  value-format="YYYY-MM-DD"
                  placeholder="选择日期"
                  style="width: 100%"
                />
              </el-form-item>
            </el-col>
            <el-col v-if="!isCreate" :span="24">
              <el-form-item label="当前结存">
                <div class="batch-stock">
                  <span class="batch-stock__main">{{ form.stockBase }} {{ baseUomName }}</span>
                  <span class="batch-stock__sep">/</span>
                  <span class="batch-stock__aux">{{ form.stockAux }} {{ auxUomName }}</span>
                </div>
              </el-form-item>
            </el-col>
            <el-col :span="24">
              <el-form-item label="备注">
                <el-input v-model="form.remark" type="textarea" :rows="2" maxlength="255" />
              </el-form-item>
            </el-col>
          </el-row>
        </el-tab-pane>

        <el-tab-pane v-if="!isCreate" label="出入库流水">
          <div class="move-head">
            <span>该批次出入库流水(账本)</span>
            <div>
              <el-button size="small" type="success" @click="openMove(1)">入库</el-button>
              <el-button size="small" type="danger" @click="openMove(2)">出库</el-button>
            </div>
          </div>
          <el-table :data="moves" size="small" border>
            <el-table-column label="类型" width="80">
              <template #default="{ row }">
                <el-tag :type="moveTypeTag(row.moveType)" size="small">
                  {{ moveTypeText(row.moveType) }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="主计量" min-width="110" align="right">
              <template #default="{ row }">{{ row.qtyBase }} {{ baseUomName }}</template>
            </el-table-column>
            <el-table-column label="辅计量" min-width="110" align="right">
              <template #default="{ row }">{{ row.qtyAux }} {{ auxUomName }}</template>
            </el-table-column>
            <el-table-column label="实际换算率" width="110" align="right">
              <template #default="{ row }">{{ row.effectiveFactor ?? '—' }}</template>
            </el-table-column>
            <el-table-column prop="creator" label="操作人" width="100" />
            <el-table-column prop="createTime" label="时间" width="160" />
            <template #empty>
              <span class="move-empty">暂无流水,点上方「入库」录入首批</span>
            </template>
          </el-table>
        </el-tab-pane>
      </el-tabs>
    </el-form>
  </AxelorDetail>

  <el-dialog v-model="moveDialog.visible" :title="moveTypeLabel" width="440px">
    <el-form label-width="90px">
      <el-form-item :label="`主计量(${baseUomName})`">
        <el-input v-model="moveDialog.qtyBase" placeholder="数量(正数)" />
      </el-form-item>
      <el-form-item :label="`辅计量(${auxUomName})`">
        <el-input v-model="moveDialog.qtyAux" placeholder="数量(正数)" />
      </el-form-item>
      <el-form-item label="备注">
        <el-input v-model="moveDialog.remark" maxlength="255" />
      </el-form-item>
      <div class="move-tip">
        两个数量都按实际录入;系统据此算本次换算率并按名义率±容差校验。
      </div>
    </el-form>
    <template #footer>
      <el-button @click="moveDialog.visible = false">取消</el-button>
      <el-button type="primary" :loading="moveSaving" @click="submitMove">确定{{ moveTypeLabel }}</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.batch-ro {
  font-weight: 600;
}
.batch-stock {
  font-size: 16px;
  font-weight: 600;
}
.batch-stock__sep {
  margin: 0 8px;
  color: var(--el-text-color-secondary);
}
.batch-stock__aux {
  color: var(--el-color-primary);
}
.move-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 4px 0 10px;
  font-weight: 600;
}
.move-empty {
  color: var(--el-text-color-secondary);
}
.move-tip {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  line-height: 1.5;
}
</style>
