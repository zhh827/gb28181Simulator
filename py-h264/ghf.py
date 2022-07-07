#!/usr/bin/python
# -*- coding: UTF-8 -*-

import random
from avc import *
 

def create_sps():
    sps = SequenceParameterSet()
    sps.header.nalu_ref_idc.value = 3
    sps.profile_idc.value = 100
    sps.level_idc.value = 30
    sps.chroma_format_idc.value = CHROMA_FORMAT_420
    sps.log2_max_pic_order_cnt_lsb_minus4.value = 2
    sps.num_ref_frames_in_pic_order_cnt_cycle.value = 4
    sps.pic_width_in_mbs_minus1.value = 43
    sps.pic_height_in_map_units_minus1.value = 35
    sps.frame_mbs_only_flag.value = 1
    sps.direct_8x8_inference_flag.value = 1
 
    sps.vui_parameters_present_flag.value = 1
    sps.vui_parameters.video_signal_type_present_flag.value = 1
    sps.vui_parameters.video_format.value = 5
    sps.vui_parameters.video_full_range_flag.value = 1
    sps.vui_parameters.timing_info_present_flag.value = 1
    sps.vui_parameters.num_units_in_tick.value = 1
    sps.vui_parameters.time_scale.value = 50
    sps.vui_parameters.bitstream_restriction_flag.value = 1
    sps.vui_parameters.log2_max_mv_length_horizontal.value = 10
    sps.vui_parameters.log2_max_mv_length_vertical.value = 10
    sps.vui_parameters.max_num_reorder_frames.value = 2
    sps.vui_parameters.max_dec_frame_buffering.value = 4
    sps.encode()
    return sps


def create_pps():
    pps = PictureParameterSet()
    pps.header.nalu_ref_idc.value = 3
    pps.entropy_coding_mode_flag.value = 0
    pps.num_ref_idx_l0_default_active_minus1.value = 2
    pps.weighted_pred_flag.value = 0
    pps.weighted_bipred_idc.value = 2
    pps.pic_init_qp_minus26.value = -3
    pps.chroma_qp_index_offset.value = -2
    pps.deblocking_filter_control_present_flag.value = 0
    pps.transform_8x8_mode_flag.value = 1
    pps.second_chroma_qp_index_offset.value = -2
    pps.encode()
    return pps


def create_sei():
    sei = SupplementalEnhancementInformation()
    # sei.header.nalu_ref_idc.value = 3
    sei.last_payload_type_byte.value = 5
    sei.user_data_unregistered.uuid_iso_iec_11587 = [FixedLenUint(32, 11111), FixedLenUint(32, 22222), FixedLenUint(32, 33333), FixedLenUint(32, 44444) ]
    sei.user_data_unregistered.user_data_payload_byte = "this is a string."
    sei.encode()
    return sei


def create_h264_stream(name="test"):
    f = open(name, "wb")
    # sps 生成
    sps = create_sps()
    print(sps.data)
    # pps 生成
    pps = create_pps()
    print(pps.data)
    # sei 生成
    sei = create_sei()
    print(sei.data)
    #
    slwr = SliceLayerWithoutPartitioningRBSP(NALU_TYPE_CODED_SLICE_IDR_PIC, 25, sps, pps)
    slwr.header.nalu_ref_idc.value = 3
    slwr.sh.first_mb_in_slice.value = 0
    slwr.sh.slice_type.value = SLICE_TYPE_I[1]
    slwr.sh.frame_num.value = 0
    slwr.sh.idr_pic_id.value = 0
    slwr.sh.disable_deblocking_filter_idc.value = 0

    # 写入 start_code + sps + start_code + pps + start_code + sei
    f.write(b"\x00\x00\x00\x01")
    f.write(sps.data)
    f.write(b"\x00\x00\x01")
    f.write(pps.data)
    f.write(b"\x00\x00\x01")
    f.write(sei.data)

    for c in range(5):
        f.write(b"\x00\x00\x01")
        # 生成一帧数据
        for m, mb in enumerate(slwr.d.macroblock_layer):
            for idx, luma in enumerate(mb.pcm_sample_luma):
                if idx < 128:
                    luma.value = c  # random.randint(0, 255)
                else:
                    luma.value = 20 * c
                if m % 2 and luma.value > 20:
                    luma.value -= 20
            if CHROMA_FORMAT_MONOCHROME != sps.chroma_format_idc.value:
                # print(len(mb.pcm_sample_chroma))
                for idx, chroma in enumerate(mb.pcm_sample_chroma):
                    chroma.value = 25 * c if idx < 64 else 10 * c  # random.randint(0, 255)
        slwr.encode()
        f.write(slwr.data)
        #  continue
        slwr1 = SliceLayerWithoutPartitioningRBSP(NALU_TYPE_CODED_SLICE_NON_IDR_PIC, 3, sps, pps)
        slwr1.header.nalu_ref_idc.value = 1
        # slwr1.sh.first_mb_in_slice.value = 0
        slwr1.sh.slice_type.value = SLICE_TYPE_P[1]
        slwr1.sh.disable_deblocking_filter_idc.value = 0
        # 1秒钟 重复24次
        for mb in range(24):
            f.write(b"\x00\x00\x01")
            slwr1.encode()
            f.write(slwr1.data)


if __name__ == "__main__":
    # create_h264_stream(name="aaa.h264")
    # f = open("aaa.ps", "wb")
    with open("aaa.h264", "rb") as f:
        data = f.read()
        for i in data:
            print(data[i:i+4])
