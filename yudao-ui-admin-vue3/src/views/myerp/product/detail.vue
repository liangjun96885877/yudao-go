<script setup lang="ts">
// myerp 产品详情/新建页 —— EAV 动态表单(myerp 的灵魂)。
//   tab1 基本信息:category / code / name / barCode / picUrl / prices / stock / status / description
//   tab2 动态属性:选 categoryId 后调 list-by-category 拉属性集合,按 inputType 动态渲染
//   右侧 Chatter(biz_type=myerp_product)
//
// 关键交互:
//   - 切换分类 → 重新拉属性 → 清空不再适用的属性值
//   - select/multi_select 的枚举选项来自属性定义的 options
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'

import { AxelorDetail } from '@/components/AxelorStyle'
import Chatter from '@/components/chatter/Chatter.vue'
import EavAttributeInput from './EavAttributeInput.vue'
import { myerpProductApi } from '@/api/myerp/product'
import { myerpCategoryApi } from '@/api/myerp/category'
import { myerpAttributeApi } from '@/api/myerp/attribute'
import { myerpUomApi } from '@/api/myerp/uom'
import { myerpStockMoveApi } from '@/api/myerp/batch'
import { useRecordNav } from '@/composables/useRecordNav'
import type {
  MyErpAttribute,
  MyErpCategory,
  MyErpProduct,
  MyErpStockMove,
  MyErpUom,
  ProductUom
} from '@/types/myerp'

defineOptions({ name: 'MyErpProductDetail' })

const route = useRoute()
const router = useRouter()

const isCreate = computed(() => route.path.endsWith('/create'))
const id = computed(() => Number(route.params.id))

const nav = useRecordNav('myerp:product')
const neighbors = computed(() => nav.neighbors(id.value))
const prevTo = computed(() =>
  neighbors.value.prev != null ? `/myerp/product/${neighbors.value.prev}` : null
)
const nextTo = computed(() =>
  neighbors.value.next != null ? `/myerp/product/${neighbors.value.next}` : null
)

const form = reactive<
  Partial<MyErpProduct> & { attrValues: Record<string, any>; uoms: ProductUom[] }
>({
  categoryId: 0,
  templateId: null,
  baseUomId: null,
  uomMode: 0,
  auxUomId: null,
  nominalFactor: '',
  tolerancePct: '0',
  batchTracked: true,
  code: '',
  name: '',
  barCode: '',
  picUrl: '',
  description: '',
  purchasePrice: '0',
  salePrice: '0',
  stock: '0',
  stockAux: '0',
  status: 0,
  ownerUserId: null,
  attrValues: {},
  uoms: []
})
const formRef = ref<FormInstance>()
const saving = ref(false)
const loading = ref(false)
const categories = ref<MyErpCategory[]>([])
const attrs = ref<MyErpAttribute[]>([])
const attrsLoading = ref(false)

// 全部启用单位(基本单位 + 辅助单位换算表共用)
const uomList = ref<MyErpUom[]>([])
const uomNameMap = computed<Record<number, string>>(() => {
  const m: Record<number, string> = {}
  for (const u of uomList.value) m[u.id] = u.name
  return m
})
// 当前基本单位名称(库存展示后缀,如 "10000 颗")
const baseUomName = computed(() =>
  form.baseUomId ? uomNameMap.value[form.baseUomId] || '' : ''
)
// 辅计量单位名称(浮动双计量)
const auxUomName = computed(() =>
  form.auxUomId ? uomNameMap.value[form.auxUomId] || '' : ''
)
// 是否浮动双计量
const isFloat = computed(() => form.uomMode === 1)
// 辅助单位下拉:排除已选基本单位 + 已在换算表里的单位
function availableAuxUoms(currentUomId?: number): MyErpUom[] {
  const used = new Set(form.uoms.map((u) => u.uomId))
  return uomList.value.filter(
    (u) => u.id === currentUomId || (u.id !== form.baseUomId && !used.has(u.id))
  )
}

async function loadUoms() {
  try {
    uomList.value = await myerpUomApi.listAll()
  } catch {
    /* ignore */
  }
}

function addUomRow() {
  form.uoms.push({ uomId: 0, factor: '1', isPurchase: false, isSale: false })
}
function removeUomRow(idx: number) {
  form.uoms.splice(idx, 1)
}

// 跳批次管理(带当前产品筛选)
function goBatches() {
  router.push(`/myerp/batch?productId=${id.value}`)
}

// === batch-less 模式:本页就地出入库 ===
const moves = ref<MyErpStockMove[]>([])
async function loadMoves() {
  if (isCreate.value || !isFloat.value || form.batchTracked) return
  try {
    const data = await myerpStockMoveApi.page({ productId: id.value, pageNo: 1, pageSize: 100 })
    moves.value = data.list
  } catch {
    moves.value = []
  }
}

const moveDialog = reactive({
  visible: false,
  moveType: 1 as number,
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
      productId: id.value,
      batchId: null as any, // batch-less:必须传 null
      moveType: moveDialog.moveType,
      qtyBase: moveDialog.qtyBase,
      qtyAux: moveDialog.qtyAux,
      remark: moveDialog.remark
    })
    ElMessage.success(`${moveTypeLabel.value}成功`)
    moveDialog.visible = false
    await loadDetail() // 刷新产品双结存 + 流水
  } catch (e: any) {
    ElMessage.error(e?.message || `${moveTypeLabel.value}失败`)
  } finally {
    moveSaving.value = false
  }
}

const moveTypeText = (t: number) => (t === 1 ? '入库' : t === 2 ? '出库' : '调整')
const moveTypeTag = (t: number) => (t === 1 ? 'success' : t === 2 ? 'danger' : 'info')

const rules: FormRules = {
  name: [{ required: true, message: '请输入产品名称', trigger: 'blur' }],
  categoryId: [
    {
      validator: (_r, v, cb) => (v > 0 ? cb() : cb(new Error('请选择分类'))),
      trigger: 'change'
    }
  ]
}

const title = computed(() =>
  isCreate.value ? '新建产品' : `产品 #${id.value} ${form.name ? `· ${form.name}` : ''}`
)

async function loadCategories() {
  try {
    categories.value = await myerpCategoryApi.tree()
  } catch {
    /* ignore */
  }
}

// 拉某分类(含继承)的属性集合,用于动态渲染。
async function loadAttrs(categoryId: number) {
  if (!categoryId) {
    attrs.value = []
    return
  }
  attrsLoading.value = true
  try {
    attrs.value = await myerpAttributeApi.listByCategory(categoryId)
  } catch (e: any) {
    ElMessage.error(e?.message || '加载属性失败')
    attrs.value = []
  } finally {
    attrsLoading.value = false
  }
}

// 切换分类:重拉属性 + 清掉不在新属性集里的旧值,并给新属性补默认空值。
async function onCategoryChange(categoryId: number) {
  await loadAttrs(categoryId)
  const next: Record<string, any> = {}
  for (const a of attrs.value) {
    // 保留旧值(同 code),否则用空/默认
    if (form.attrValues[a.code] !== undefined) {
      next[a.code] = form.attrValues[a.code]
    } else {
      next[a.code] = a.inputType === 'multi_select' ? [] : a.inputType === 'bool' ? false : ''
    }
  }
  form.attrValues = next
}

async function loadDetail() {
  if (isCreate.value) return
  loading.value = true
  try {
    const p = await myerpProductApi.get(id.value)
    Object.assign(form, p)
    if (!form.attrValues) form.attrValues = {}
    form.uoms = (p.uoms || []).map((u) => ({ ...u }))
    await loadMoves()
    // 加载该产品分类的属性定义(用于渲染输入控件 + 枚举)
    await loadAttrs(p.categoryId)
    // 把已存的值规整到 attrValues(multi_select 的 JSON 字符串要解析回数组)
    const normalized: Record<string, any> = {}
    for (const a of attrs.value) {
      const raw = (p.attrs || {})[a.code]
      if (a.inputType === 'multi_select') {
        normalized[a.code] = parseMulti(raw)
      } else if (a.inputType === 'bool') {
        normalized[a.code] = raw === 'true' || raw === true || raw === '1'
      } else {
        normalized[a.code] = raw ?? ''
      }
    }
    form.attrValues = normalized
  } catch (e: any) {
    ElMessage.error(e?.message || '加载产品失败')
  } finally {
    loading.value = false
  }
}

function parseMulti(raw: any): string[] {
  if (Array.isArray(raw)) return raw
  if (typeof raw === 'string' && raw.startsWith('[')) {
    try {
      return JSON.parse(raw)
    } catch {
      return []
    }
  }
  return raw ? [String(raw)] : []
}

async function onSave() {
  if (!formRef.value) return
  try {
    await formRef.value.validate()
  } catch {
    return
  }
  // 组装 attrValues:空值不提交(让后端按必填规则校验)
  const payloadAttrs: Record<string, any> = {}
  for (const a of attrs.value) {
    const v = form.attrValues[a.code]
    if (v === '' || v === null || v === undefined) continue
    if (Array.isArray(v) && v.length === 0) continue
    payloadAttrs[a.code] = v
  }
  // 多单位换算:只提交选了单位的行
  const payloadUoms = form.uoms
    .filter((u) => u.uomId > 0)
    .map((u) => ({
      uomId: u.uomId,
      factor: u.factor || '1',
      isPurchase: !!u.isPurchase,
      isSale: !!u.isSale
    }))
  saving.value = true
  try {
    const body = {
      categoryId: form.categoryId,
      templateId: form.templateId,
      baseUomId: form.baseUomId,
      uomMode: form.uomMode,
      auxUomId: isFloat.value ? form.auxUomId : null,
      nominalFactor: isFloat.value && form.nominalFactor ? form.nominalFactor : null,
      tolerancePct: form.tolerancePct || '0',
      batchTracked: isFloat.value ? !!form.batchTracked : true,
      code: form.code,
      name: form.name,
      barCode: form.barCode,
      picUrl: form.picUrl,
      description: form.description,
      purchasePrice: form.purchasePrice,
      salePrice: form.salePrice,
      stock: form.stock,
      status: form.status,
      ownerUserId: form.ownerUserId,
      attrValues: payloadAttrs,
      uoms: payloadUoms
    }
    if (isCreate.value) {
      const newId = await myerpProductApi.create(body)
      ElMessage.success('创建成功')
      router.replace(`/myerp/product/${newId}`)
    } else {
      await myerpProductApi.update({ ...body, id: id.value })
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
    await myerpProductApi.delete(id.value)
    ElMessage.success('已删除')
    router.replace('/myerp/product')
  } catch (e: any) {
    ElMessage.error(e?.message || '删除失败')
  }
}

// 复制为新:带主表字段 + 分类 + EAV 属性值进新建页,code 清空
function onCopy() {
  const draft = { ...form, code: '' }
  sessionStorage.setItem('myerp:product:copy', JSON.stringify(draft))
  router.push('/myerp/product/create')
}
function onRefresh() {
  loadDetail()
}

onMounted(async () => {
  await Promise.all([loadCategories(), loadUoms()])
  if (isCreate.value) {
    const raw = sessionStorage.getItem('myerp:product:copy')
    if (raw) {
      try {
        const draft = JSON.parse(raw)
        Object.assign(form, draft)
        // 复制带了分类 → 拉该分类属性集以渲染动态表单
        if (draft.categoryId) await loadAttrs(draft.categoryId)
      } catch {
        /* ignore */
      }
      sessionStorage.removeItem('myerp:product:copy')
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
    :show-chatter="!isCreate"
    :show-delete="!isCreate"
    :show-copy="!isCreate"
    :show-refresh="!isCreate"
    :prev-to="isCreate ? undefined : prevTo"
    :next-to="isCreate ? undefined : nextTo"
    :nav-index="isCreate ? -1 : neighbors.index"
    :nav-total="neighbors.total"
    create-route="/myerp/product/create"
    back-to="/myerp/product"
    @save="onSave"
    @delete="onDelete"
    @copy="onCopy"
    @refresh="onRefresh"
  >
    <el-form
      ref="formRef"
      :model="form"
      :rules="rules"
      label-width="90px"
      v-loading="loading"
    >
      <el-alert
        v-if="form.templateId"
        type="warning"
        :closable="false"
        show-icon
        class="prod-tpl-tip"
      >
        本 SKU 属于模板
        <el-link
          type="primary"
          :underline="false"
          @click="$router.push(`/myerp/template/${form.templateId}`)"
        >「{{ form.templateName || `#${form.templateId}` }}」</el-link>
        共享字段(名称/分类/单位/基础售价/区分属性)请回模板修改;本页只编辑此 SKU 独有字段。
      </el-alert>
      <el-tabs>
        <!-- ① 基本信息 -->
        <el-tab-pane label="基本信息">
          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="分类" prop="categoryId">
                <el-select
                  v-model="form.categoryId"
                  placeholder="请选择分类"
                  filterable
                  @change="onCategoryChange"
                >
                  <el-option
                    v-for="c in categories"
                    :key="c.id"
                    :value="c.id"
                    :label="`${c.name} (#${c.id})`"
                  />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="名称" prop="name">
                <el-input v-model="form.name" placeholder="如:iPhone 15 Pro" maxlength="128" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="编码">
                <el-input v-model="form.code" placeholder="SKU 编码" maxlength="64" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="条形码">
                <el-input v-model="form.barCode" maxlength="64" />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="采购价">
                <el-input v-model="form.purchasePrice" placeholder="0.00">
                  <template #prepend>¥</template>
                </el-input>
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="销售价">
                <el-input v-model="form.salePrice" placeholder="0.00">
                  <template #prepend>¥</template>
                </el-input>
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="库存">
                <el-input
                  v-model="form.stock"
                  :disabled="isFloat"
                  :placeholder="isFloat ? '由批次出入库管理' : '0'"
                >
                  <template v-if="baseUomName" #append>{{ baseUomName }}</template>
                </el-input>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="图片 URL">
                <el-input v-model="form.picUrl" placeholder="https://" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="状态">
                <el-radio-group v-model="form.status">
                  <el-radio :value="0">启用</el-radio>
                  <el-radio :value="1">停用</el-radio>
                </el-radio-group>
              </el-form-item>
            </el-col>
            <el-col :span="24">
              <el-form-item label="描述">
                <el-input
                  v-model="form.description"
                  type="textarea"
                  :rows="2"
                  maxlength="1024"
                  show-word-limit
                />
              </el-form-item>
            </el-col>
          </el-row>
        </el-tab-pane>

        <!-- ② 动态属性(EAV) -->
        <el-tab-pane label="属性">
          <div v-if="!form.categoryId" class="ax-empty-tip">请先在「基本信息」选择分类</div>
          <div v-else-if="attrs.length === 0 && !attrsLoading" class="ax-empty-tip">
            该分类暂无属性定义
          </div>
          <el-row v-else :gutter="16" v-loading="attrsLoading">
            <el-col v-for="a in attrs" :key="a.code" :span="12">
              <el-form-item>
                <template #label>
                  <span>
                    {{ a.name }}
                    <span v-if="a.required" class="ax-required">*</span>
                    <el-tag size="small" type="info" class="ax-attr-type">{{ a.inputType }}</el-tag>
                  </span>
                </template>
                <EavAttributeInput :attr="a" v-model="form.attrValues[a.code]" />
              </el-form-item>
            </el-col>
          </el-row>
        </el-tab-pane>

        <!-- ③ 多单位换算 -->
        <el-tab-pane label="多单位">
          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="基本单位">
                <el-select
                  v-model="form.baseUomId"
                  placeholder="库存/计价的最小单位(如:颗)"
                  clearable
                  filterable
                >
                  <el-option
                    v-for="u in uomList"
                    :key="u.id"
                    :value="u.id"
                    :label="`${u.name}${u.code ? ` (${u.code})` : ''}`"
                  />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="换算方式">
                <el-radio-group v-model="form.uomMode">
                  <el-radio :value="0">固定</el-radio>
                  <el-radio :value="1">浮动双计量</el-radio>
                </el-radio-group>
              </el-form-item>
            </el-col>
          </el-row>

          <!-- 浮动双计量配置 -->
          <el-alert
            v-if="isFloat"
            type="info"
            :closable="false"
            show-icon
            class="uom-float-tip"
          >
            浮动双计量:主、辅两个数量独立记账(如「个 / 克」),换算率因批而异,
            实际换算率与库存经「批次 + 出入库」管理。
          </el-alert>
          <el-row v-if="isFloat" :gutter="16">
            <el-col :span="8">
              <el-form-item label="辅计量单位">
                <el-select
                  v-model="form.auxUomId"
                  placeholder="异重单位(如:克)"
                  clearable
                  filterable
                >
                  <el-option
                    v-for="u in uomList.filter((x) => x.id !== form.baseUomId)"
                    :key="u.id"
                    :value="u.id"
                    :label="`${u.name}${u.code ? ` (${u.code})` : ''}`"
                  />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="名义换算率">
                <el-input v-model="form.nominalFactor" placeholder="如 10">
                  <template #prepend>1 {{ baseUomName || '主' }} ≈</template>
                  <template #append>{{ auxUomName || '辅' }}</template>
                </el-input>
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="允许偏差">
                <el-input v-model="form.tolerancePct" placeholder="0=不校验">
                  <template #append>%</template>
                </el-input>
              </el-form-item>
            </el-col>
            <el-col :span="24">
              <el-form-item label="按批次管理">
                <el-switch v-model="form.batchTracked" />
                <span class="uom-hint">
                  开启:按批管理(肉类/农产品每批均率);
                  关闭:batch-less 随机重量,**每笔都不同**(生鲜过磅/钢板/散装/废料)
                </span>
              </el-form-item>
            </el-col>
            <el-col v-if="!isCreate" :span="24">
              <el-form-item label="当前库存">
                <span class="uom-stock">
                  {{ form.stock || 0 }} <span class="uom-stock__unit">{{ baseUomName }}</span>
                  <span class="uom-stock__sep">/</span>
                  {{ form.stockAux || 0 }} <span class="uom-stock__unit">{{ auxUomName }}</span>
                </span>
                <el-button
                  v-if="form.batchTracked"
                  link type="primary" class="uom-batch-link" @click="goBatches"
                >去批次管理 →</el-button>
                <span v-else class="uom-batchless-tag">
                  <el-tag size="small" type="warning">batch-less</el-tag>
                </span>
              </el-form-item>
            </el-col>
            <!-- batch-less:就地出入库 + 流水(无需建批次) -->
            <el-col v-if="!isCreate && !form.batchTracked" :span="24">
              <div class="uom-move-head">
                <span>出入库流水(账本 · batch-less)</span>
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
                  <span class="uom-hint">暂无流水,点上方「入库」录入首笔(每笔都按实际称量录入)</span>
                </template>
              </el-table>
            </el-col>
          </el-row>
          <el-row v-else :gutter="16">
            <el-col :span="12">
              <el-form-item label="当前库存">
                <span class="uom-stock">
                  {{ form.stock || 0 }}
                  <span class="uom-stock__unit">{{ baseUomName || '(未设基本单位)' }}</span>
                </span>
              </el-form-item>
            </el-col>
          </el-row>

          <!-- 固定模式才显示包装单位换算表;浮动产品语义弱,直接隐藏 -->
          <div v-if="!isFloat" class="uom-aux-head">
            <span>采购/销售包装单位换算</span>
            <el-button size="small" type="primary" plain @click="addUomRow">+ 添加单位</el-button>
          </div>
          <el-table v-if="!isFloat" :data="form.uoms" size="small" border class="uom-aux-table">
            <el-table-column label="辅助单位" min-width="160">
              <template #default="{ row }">
                <el-select v-model="row.uomId" placeholder="选择单位" filterable style="width: 100%">
                  <el-option
                    v-for="u in availableAuxUoms(row.uomId)"
                    :key="u.id"
                    :value="u.id"
                    :label="`${u.name}${u.code ? ` (${u.code})` : ''}`"
                  />
                </el-select>
              </template>
            </el-table-column>
            <el-table-column label="换算系数" min-width="220">
              <template #default="{ row }">
                <div class="uom-factor">
                  <span class="uom-factor__lead">1 {{ uomNameMap[row.uomId] || '辅助单位' }} =</span>
                  <el-input v-model="row.factor" placeholder="50" style="width: 100px" />
                  <span class="uom-factor__tail">{{ baseUomName || '基本单位' }}</span>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="默认采购" width="90" align="center">
              <template #default="{ row }">
                <el-checkbox v-model="row.isPurchase" />
              </template>
            </el-table-column>
            <el-table-column label="默认销售" width="90" align="center">
              <template #default="{ row }">
                <el-checkbox v-model="row.isSale" />
              </template>
            </el-table-column>
            <el-table-column label="操作" width="70" align="center">
              <template #default="{ $index }">
                <el-button link type="danger" @click="removeUomRow($index)">删除</el-button>
              </template>
            </el-table-column>
            <template #empty>
              <span class="ax-empty-tip">暂无辅助单位,如「1 斤 = 50 颗」点上方「+ 添加单位」</span>
            </template>
          </el-table>
        </el-tab-pane>
      </el-tabs>
    </el-form>

    <template #chatter>
      <Chatter v-if="!isCreate && id" biz-type="myerp_product" :biz-id="id" />
    </template>
  </AxelorDetail>

  <!-- batch-less 出入库 dialog -->
  <el-dialog v-model="moveDialog.visible" :title="moveTypeLabel + ' (batch-less)'" width="440px">
    <el-form label-width="100px">
      <el-form-item :label="`主计量(${baseUomName})`">
        <el-input v-model="moveDialog.qtyBase" placeholder="本笔实际数量(正数)" />
      </el-form-item>
      <el-form-item :label="`辅计量(${auxUomName})`">
        <el-input v-model="moveDialog.qtyAux" placeholder="本笔实际数量(正数)" />
      </el-form-item>
      <el-form-item label="备注">
        <el-input v-model="moveDialog.remark" maxlength="255" />
      </el-form-item>
      <div class="uom-hint">
        每笔都按实际称量录入,系统据此算本笔换算率并按名义率±容差校验。
      </div>
    </el-form>
    <template #footer>
      <el-button @click="moveDialog.visible = false">取消</el-button>
      <el-button type="primary" :loading="moveSaving" @click="submitMove">
        确定{{ moveTypeLabel }}
      </el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.ax-empty-tip {
  padding: 40px;
  text-align: center;
  color: var(--el-text-color-secondary);
}
.ax-required {
  color: var(--el-color-danger);
  margin: 0 2px;
}
.ax-attr-type {
  margin-left: 4px;
  transform: scale(0.85);
}
.uom-stock {
  font-size: 16px;
  font-weight: 600;
}
.uom-stock__unit {
  margin-left: 4px;
  font-size: 13px;
  font-weight: 400;
  color: var(--el-text-color-secondary);
}
.uom-stock__sep {
  margin: 0 10px;
  color: var(--el-text-color-secondary);
}
.uom-batch-link {
  margin-left: 14px;
}
.uom-float-tip {
  margin-bottom: 14px;
}
.uom-hint {
  margin-left: 10px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
.uom-batchless-tag {
  margin-left: 14px;
}
.uom-move-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 16px 0 10px;
  font-weight: 600;
}
.prod-tpl-tip {
  margin-bottom: 14px;
}
.uom-aux-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 4px 0 10px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}
.uom-aux-table {
  width: 100%;
}
.uom-factor {
  display: flex;
  align-items: center;
  gap: 6px;
}
.uom-factor__lead,
.uom-factor__tail {
  color: var(--el-text-color-secondary);
  white-space: nowrap;
}
</style>
